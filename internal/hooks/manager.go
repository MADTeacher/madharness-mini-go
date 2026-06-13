package hooks

import (
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// TraceWriter — минимальный контракт trace, нужный dispatcher-у hooks.
type TraceWriter interface {
	Write(event string, fields map[string]any) error
}

// TraceRef описывает trace-файл текущей сессии без привязки к concrete type.
type TraceRef interface {
	TraceWriter
	TraceID() string
}

// Manager синхронно вызывает подходящие hooks и пишет результат в trace.
type Manager struct {
	providers []Provider
	trace     TraceRef
}

// NewManager строит dispatcher поверх уже готовых providers.
func NewManager(providers []Provider, trace TraceRef) *Manager {
	return &Manager{providers: append([]Provider{}, providers...), trace: trace}
}

// FromConfig читает `.madharness-mini/hooks.json` и создаёт command providers.
func FromConfig(cfg *config.Config, trace TraceRef) (*Manager, error) {
	configs, err := LoadCommandConfigs(cfg)
	if err != nil {
		return nil, err
	}
	providers := make([]Provider, 0, len(configs))
	for _, config := range configs {
		providers = append(providers, CommandProvider{Config: config})
	}
	return NewManager(providers, trace), nil
}

// WithTrace переиспользует тот же набор hooks для дочерней трассы субагента.
func (m *Manager) WithTrace(trace TraceRef) *Manager {
	if m == nil {
		return nil
	}
	return NewManager(m.providers, trace)
}

// Emit отправляет lifecycle-событие подходящим hooks по порядку.
func (m *Manager) Emit(name string, kind string, data map[string]any) Decision {
	if m == nil || len(m.providers) == 0 {
		return Allow()
	}
	traceID := ""
	if m.trace != nil {
		traceID = m.trace.TraceID()
	}
	event := Event{
		Name:    name,
		Kind:    kind,
		TraceID: traceID,
		Data:    compactMap(data),
	}
	for _, provider := range m.providers {
		if !provider.Matches(event) {
			continue
		}
		decision := m.handle(provider, event)
		if !decision.OK {
			return decision
		}
	}
	return Allow()
}

func (m *Manager) handle(provider Provider, event Event) Decision {
	started := time.Now()
	m.write("hook_started", map[string]any{
		"hook":       provider.ID(),
		"hook_event": event.Name,
		"kind":       event.Kind,
	})
	decision, err := provider.Handle(event)
	if err != nil {
		m.write("hook_failed", map[string]any{
			"hook":       provider.ID(),
			"hook_event": event.Name,
			"kind":       event.Kind,
			"error":      tools.Clipped(err.Error(), 1000),
			"elapsed_ms": elapsedMS(started),
		})
		return Allow()
	}
	if decision.OK {
		m.write("hook_finished", map[string]any{
			"hook":       provider.ID(),
			"hook_event": event.Name,
			"kind":       event.Kind,
			"message":    tools.Clipped(decision.Message, 1000),
			"elapsed_ms": elapsedMS(started),
		})
		return decision
	}
	m.write("hook_blocked", map[string]any{
		"hook":       provider.ID(),
		"hook_event": event.Name,
		"kind":       event.Kind,
		"block":      tools.Clipped(decision.Block, 1000),
		"message":    tools.Clipped(decision.Message, 1000),
		"elapsed_ms": elapsedMS(started),
	})
	return decision
}

func (m *Manager) write(event string, fields map[string]any) {
	if m.trace != nil {
		_ = m.trace.Write(event, fields)
	}
}

func compactMap(data map[string]any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if compact, ok := CompactPayload(data).(map[string]any); ok {
		return compact
	}
	return map[string]any{}
}

func elapsedMS(started time.Time) int {
	return int(time.Since(started).Milliseconds())
}
