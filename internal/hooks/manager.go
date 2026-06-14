package hooks

import (
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// ObserveQueueSize ограничивает backlog асинхронных observe hooks.
const ObserveQueueSize = 32

// TraceWriter — минимальный контракт trace, нужный dispatcher-у hooks.
type TraceWriter = events.TraceWriter

// TraceRef описывает trace-файл текущей сессии без привязки к concrete type.
type TraceRef = events.TraceRef

// Manager вызывает enforce hooks синхронно, а observe hooks отправляет в очередь.
type Manager struct {
	enforce []Provider
	observe []Provider
	trace   TraceRef
	queue   *observeQueue
}

// NewManager строит dispatcher поверх уже готовых providers.
func NewManager(providers []Provider, trace TraceRef) *Manager {
	return newManager(providers, nil, trace)
}

// FromConfig читает `.madharness-mini/hooks.json` и создаёт command providers.
func FromConfig(cfg *config.Config, trace TraceRef) (*Manager, error) {
	configs, err := LoadCommandConfigs(cfg)
	if err != nil {
		return nil, err
	}
	enforce := []Provider{}
	observe := []Provider{}
	for _, config := range configs {
		provider := CommandProvider{Config: config}
		if config.Mode == ModeObserve {
			observe = append(observe, provider)
		} else {
			enforce = append(enforce, provider)
		}
	}
	return newManager(enforce, observe, trace), nil
}

func newManager(enforce []Provider, observe []Provider, trace TraceRef) *Manager {
	manager := &Manager{
		enforce: append([]Provider{}, enforce...),
		observe: append([]Provider{}, observe...),
		trace:   trace,
	}
	if len(manager.observe) > 0 {
		manager.queue = newObserveQueue(manager.handle)
	}
	return manager
}

// WithTrace переиспользует тот же набор hooks для дочерней трассы субагента.
func (m *Manager) WithTrace(trace TraceRef) events.Subscriber {
	if m == nil {
		return nil
	}
	return newManager(m.enforce, m.observe, trace)
}

// Close дожидается observe hooks, которые уже попали в очередь.
func (m *Manager) Close() error {
	if m == nil || m.queue == nil {
		return nil
	}
	m.queue.Close()
	return nil
}

// Publish отправляет lifecycle-событие подходящим hooks.
func (m *Manager) Publish(event events.Event) events.Decision {
	if m == nil || len(m.enforce)+len(m.observe) == 0 {
		return Allow()
	}
	event = m.prepare(event)
	blocked := Allow()
	for _, provider := range m.enforce {
		if !provider.Matches(event) {
			continue
		}
		decision := m.handle(provider, event)
		if !decision.OK && canBlock(event.Name) {
			blocked = decision
			break
		}
	}
	m.enqueueObserve(event)
	return blocked
}

func canBlock(eventName string) bool {
	return eventName == "before_tool_call" || eventName == "approval_request"
}

// Emit отправляет lifecycle-событие подходящим hooks по порядку.
func (m *Manager) Emit(name string, kind string, data map[string]any) Decision {
	return m.Publish(events.Event{Name: name, Kind: kind, HookData: data})
}

func (m *Manager) prepare(event events.Event) events.Event {
	traceID := ""
	if m.trace != nil {
		traceID = m.trace.TraceID()
	}
	if event.TraceID == "" {
		event.TraceID = traceID
	}
	event.HookData = compactMap(event.HookData)
	return event
}

func (m *Manager) enqueueObserve(event Event) {
	if len(m.observe) == 0 {
		return
	}
	for _, provider := range m.observe {
		if !provider.Matches(event) {
			continue
		}
		if m.queue != nil && m.queue.Enqueue(observeJob{provider: provider, event: event}) {
			continue
		}
		m.write("hook_failed", map[string]any{
			"hook":       provider.ID(),
			"hook_event": event.Name,
			"kind":       event.Kind,
			"error":      "observe hook queue full",
			"elapsed_ms": 0,
		})
	}
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
	decision = withSource(decision, provider.ID())
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

func withSource(decision Decision, source string) Decision {
	if decision.Source == "" || decision.Source == "hook" {
		decision.Source = source
	}
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
