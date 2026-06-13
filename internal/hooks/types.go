// Package hooks реализует lifecycle hooks проекта вокруг агентского цикла.
package hooks

// SchemaVersion фиксирует версию JSON-контракта события для hook-команды.
const SchemaVersion = 1

// Events перечисляет события жизненного цикла, доступные проектным hooks.
var Events = map[string]bool{
	"session_start":     true,
	"before_model_call": true,
	"after_model_call":  true,
	"before_tool_call":  true,
	"after_tool_call":   true,
	"session_end":       true,
	"session_error":     true,
}

// Event описывает одно событие harness, которое hook получает через stdin.
type Event struct {
	Name    string
	Kind    string
	TraceID string
	Data    map[string]any
}

// Map возвращает стабильный JSON-совместимый payload для hook-команды.
func (e Event) Map() map[string]any {
	return map[string]any{
		"version":  SchemaVersion,
		"event":    e.Name,
		"kind":     e.Kind,
		"trace_id": e.TraceID,
		"data":     e.Data,
	}
}

// Decision хранит решение hook: продолжить выполнение или заблокировать шаг.
type Decision struct {
	OK      bool
	Block   string
	Message string
}

// Allow возвращает нейтральное решение, которое не меняет ход agent loop.
func Allow() Decision {
	return Decision{OK: true}
}

// Provider обрабатывает одно lifecycle-событие.
type Provider interface {
	ID() string
	Matches(Event) bool
	Handle(Event) (Decision, error)
}
