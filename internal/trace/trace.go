// Package trace пишет JSONL-трассы запусков ask/run.
package trace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/redaction"
)

// Trace представляет один файл событий текущей сессии.
type Trace struct {
	ID            string
	Path          string
	mu            sync.Mutex
	seq           int64
	spanSeq       int64
	sessionSpanID string
	sessionSpan   *Span
}

// Span описывает открытую диагностическую область внутри trace.
type Span struct {
	trace     *Trace
	id        string
	name      string
	startedAt time.Time
	endOnce   sync.Once
}

// ScopedTrace пишет события в тот же JSONL-файл, но привязывает их к span.
type ScopedTrace struct {
	trace  *Trace
	spanID string
}

// TraceID отдаёт стабильный идентификатор текущей трассы для lifecycle hooks.
func (t *Trace) TraceID() string {
	if t == nil {
		return ""
	}
	return t.ID
}

// SessionSpanID возвращает span всей agent session.
func (t *Trace) SessionSpanID() string {
	if t == nil {
		return ""
	}
	return t.sessionSpanID
}

// New создаёт trace-файл и сразу пишет session_start.
func New(cfg *config.Config, kind string) (*Trace, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	id := time.Now().Format("20060102-150405") + "-" + randomSuffix()
	dir := filepath.Join(cfg.StateDir, "traces", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tr := &Trace{
		ID:   id,
		Path: filepath.Join(dir, id+".jsonl"),
	}
	session := tr.StartSpan("session", "", map[string]any{"kind": kind})
	tr.sessionSpanID = session.ID()
	tr.sessionSpan = session
	if err := tr.Write("session_start", map[string]any{"kind": kind}); err != nil {
		return nil, err
	}
	return tr, nil
}

// Child создаёт trace дочернего запуска рядом с parent trace.
func (t *Trace) Child(kind string, label string) (*Trace, error) {
	return t.ChildWithParent(kind, label, "", "")
}

// ChildWithParent создаёт дочернюю trace и связывает её с parent span/tool call.
func (t *Trace) ChildWithParent(kind string, label string, parentSpanID string, parentToolCallID string) (*Trace, error) {
	id := t.ID + "--" + safeTraceLabel(label) + "-" + randomSuffix()
	tr := &Trace{
		ID:   id,
		Path: filepath.Join(filepath.Dir(t.Path), id+".jsonl"),
	}
	fields := map[string]any{
		"kind":            kind,
		"parent_trace_id": t.ID,
		"label":           label,
	}
	if parentSpanID != "" {
		fields["parent_span_id"] = parentSpanID
	}
	if parentToolCallID != "" {
		fields["parent_tool_call_id"] = parentToolCallID
	}
	session := tr.StartSpan("session", parentSpanID, fields)
	tr.sessionSpanID = session.ID()
	tr.sessionSpan = session
	startFields := map[string]any{
		"kind":      kind,
		"parent_id": t.ID,
		"label":     label,
	}
	for key, value := range fields {
		startFields[key] = value
	}
	if err := tr.Write("session_start", startFields); err != nil {
		return nil, err
	}
	return tr, nil
}

func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf)
}

func safeTraceLabel(value string) string {
	var builder strings.Builder
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' {
			builder.WriteRune(ch)
		} else {
			builder.WriteRune('-')
		}
	}
	parts := strings.FieldsFunc(builder.String(), func(ch rune) bool { return ch == '-' })
	cleaned := strings.Join(parts, "-")
	if cleaned == "" {
		return "child"
	}
	if len(cleaned) > 80 {
		return cleaned[:80]
	}
	return cleaned
}

// Write дописывает одно событие в JSONL.
func (t *Trace) Write(event string, fields map[string]any) error {
	return t.write(event, fields, t.SessionSpanID(), "")
}

// StartSpan открывает span и сразу пишет span_start в trace.
func (t *Trace) StartSpan(name string, parentSpanID string, fields map[string]any) *Span {
	if t == nil {
		return &Span{}
	}
	spanID := t.nextSpanID()
	if parentSpanID == "" && name != "session" {
		parentSpanID = t.SessionSpanID()
	}
	started := time.Now()
	data := cloneFields(fields)
	data["name"] = name
	_ = t.write("span_start", data, spanID, parentSpanID)
	return &Span{trace: t, id: spanID, name: name, startedAt: started}
}

// WithSpan создаёт scoped writer для уже известного span.
func (t *Trace) WithSpan(spanID string) *ScopedTrace {
	return &ScopedTrace{trace: t, spanID: spanID}
}

// Flush явно завершает запись trace. Сейчас Write открывает файл на каждую строку,
// поэтому сбрасывать нечего, но контракт нужен владельцам session lifecycle.
func (t *Trace) Flush() error {
	return nil
}

// EndSessionSpan закрывает span всей agent session.
func (t *Trace) EndSessionSpan(status string, fields map[string]any) {
	if t == nil || t.sessionSpan == nil {
		return
	}
	t.sessionSpan.End(status, fields)
}

func (t *Trace) nextSpanID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spanSeq++
	return t.ID + ":span-" + strconv.FormatInt(t.spanSeq, 10)
}

func (t *Trace) write(event string, fields map[string]any, spanID string, parentSpanID string) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	record := map[string]any{
		"ts":       float64(time.Now().UnixNano()) / 1e9,
		"event":    event,
		"seq":      t.seq,
		"event_id": t.ID + ":" + strconv.FormatInt(t.seq, 10),
	}
	if spanID != "" {
		record["span_id"] = spanID
	}
	if parentSpanID != "" {
		record["parent_span_id"] = parentSpanID
	}
	for key, value := range fields {
		record[key] = value
	}
	record = redaction.RedactPayload(record).(map[string]any)
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(t.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// ID возвращает идентификатор span для передачи дочерним событиям.
func (s *Span) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// Trace возвращает scoped writer текущего span.
func (s *Span) Trace() *ScopedTrace {
	if s == nil {
		return nil
	}
	return s.trace.WithSpan(s.id)
}

// End закрывает span и пишет span_end один раз.
func (s *Span) End(status string, fields map[string]any) {
	if s == nil || s.trace == nil || s.id == "" {
		return
	}
	s.endOnce.Do(func() {
		if status == "" {
			status = "ok"
		}
		data := cloneFields(fields)
		data["name"] = s.name
		data["status"] = status
		data["elapsed_ms"] = int(time.Since(s.startedAt).Milliseconds())
		_ = s.trace.write("span_end", data, s.id, "")
	})
}

// Write дописывает событие в span scoped writer.
func (s *ScopedTrace) Write(event string, fields map[string]any) error {
	if s == nil || s.trace == nil {
		return nil
	}
	return s.trace.write(event, fields, s.spanID, "")
}

// TraceID отдаёт идентификатор underlying trace.
func (s *ScopedTrace) TraceID() string {
	if s == nil || s.trace == nil {
		return ""
	}
	return s.trace.TraceID()
}

// WithSpan пересоздаёт scoped writer для вложенного span.
func (s *ScopedTrace) WithSpan(spanID string) *ScopedTrace {
	if s == nil {
		return nil
	}
	return s.trace.WithSpan(spanID)
}

// StartSpan открывает дочерний span относительно текущего scoped span.
func (s *ScopedTrace) StartSpan(name string, parentSpanID string, fields map[string]any) *Span {
	if s == nil || s.trace == nil {
		return &Span{}
	}
	if parentSpanID == "" {
		parentSpanID = s.spanID
	}
	return s.trace.StartSpan(name, parentSpanID, fields)
}

func cloneFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range fields {
		out[key] = value
	}
	return out
}
