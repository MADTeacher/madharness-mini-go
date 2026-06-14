// Package hooks реализует lifecycle hooks проекта вокруг агентского цикла.
package hooks

import "github.com/MADTeacher/madharness-mini-go/internal/events"

// SchemaVersion фиксирует версию JSON-контракта события для hook-команды.
const SchemaVersion = 1

const (
	// ModeEnforce оставляет hook на синхронном критическом пути.
	ModeEnforce = "enforce"
	// ModeObserve запускает hook асинхронно без права блокировать действие.
	ModeObserve = "observe"
)

// Events перечисляет события жизненного цикла, доступные проектным hooks.
var Events = map[string]bool{
	"session_start":     true,
	"before_model_call": true,
	"after_model_call":  true,
	"before_tool_call":  true,
	"after_tool_call":   true,
	"approval_request":  true,
	"approval_decision": true,
	"session_end":       true,
	"session_error":     true,
}

// Event описывает одно событие harness, которое hook получает через stdin.
type Event = events.Event

// Decision хранит решение hook: продолжить выполнение или заблокировать шаг.
type Decision = events.Decision

// Allow возвращает нейтральное решение, которое не меняет ход agent loop.
func Allow() Decision {
	return events.Allow()
}

// Provider обрабатывает одно lifecycle-событие.
type Provider interface {
	ID() string
	Matches(Event) bool
	Handle(Event) (Decision, error)
}
