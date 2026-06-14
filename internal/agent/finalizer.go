package agent

import (
	"sync"

	"github.com/MADTeacher/madharness-mini-go/internal/events"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// sessionFinalizer централизует terminal lifecycle agent session.
type sessionFinalizer struct {
	kind     string
	tr       *trace.Trace
	events   *events.Bus
	cleanups []func()
	once     sync.Once
}

func newSessionFinalizer(kind string, tr *trace.Trace, bus *events.Bus) *sessionFinalizer {
	return &sessionFinalizer{kind: kind, tr: tr, events: bus}
}

func (f *sessionFinalizer) AddCleanup(cleanup func()) {
	if f == nil || cleanup == nil {
		return
	}
	f.cleanups = append(f.cleanups, cleanup)
}

func (f *sessionFinalizer) Finish(status string, result string, turns int, hookData map[string]any) {
	if f == nil {
		return
	}
	f.once.Do(func() {
		f.runCleanups()
		data := cloneHookData(hookData)
		if status != "" {
			data["status"] = status
		}
		if turns > 0 {
			data["turns"] = turns
		}
		if _, ok := data["result_preview"]; !ok {
			data["result_preview"] = truncateForHook(result, 1000)
		}
		publishEvent(f.events, events.Event{
			Name:      "session_end",
			Kind:      f.kind,
			SpanID:    f.sessionSpanID(),
			HookData:  data,
			TraceName: "session_end",
			TraceData: map[string]any{"result": result},
		})
		f.close("ok", map[string]any{"status": status})
	})
}

func (f *sessionFinalizer) Fail(err error, turn any) {
	if f == nil || err == nil {
		return
	}
	f.once.Do(func() {
		f.runCleanups()
		data := map[string]any{
			"error_type": errorType(err),
			"message":    err.Error(),
		}
		if turn != nil {
			data["turn"] = turn
		}
		publishEvent(f.events, events.Event{
			Name:      "session_error",
			Kind:      f.kind,
			SpanID:    f.sessionSpanID(),
			HookData:  data,
			TraceName: "session_error",
			TraceData: data,
		})
		if f.tr != nil {
			_ = f.tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		}
		f.close("error", map[string]any{"error": err.Error()})
	})
}

func (f *sessionFinalizer) TraceOnlyError(err error) {
	if f == nil || err == nil {
		return
	}
	f.once.Do(func() {
		if f.tr != nil {
			_ = f.tr.Write("session_error", map[string]any{
				"error_type": errorType(err),
				"message":    err.Error(),
			})
			_ = f.tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		}
		f.close("error", map[string]any{"error": err.Error()})
	})
}

func (f *sessionFinalizer) runCleanups() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

func (f *sessionFinalizer) close(status string, fields map[string]any) {
	if f.events != nil {
		_ = f.events.Close()
	}
	if f.tr != nil {
		f.tr.EndSessionSpan(status, fields)
		_ = f.tr.Flush()
	}
}

func (f *sessionFinalizer) sessionSpanID() string {
	if f == nil || f.tr == nil {
		return ""
	}
	return f.tr.SessionSpanID()
}

func cloneHookData(data map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range data {
		out[key] = value
	}
	return out
}
