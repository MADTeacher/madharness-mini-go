// Package events описывает внутреннюю шину lifecycle-событий harness.
package events

// Event описывает одно lifecycle-событие для внутренних подписчиков.
type Event struct {
	Name      string
	Kind      string
	TraceID   string
	SpanID    string
	HookData  map[string]any
	TraceName string
	TraceData map[string]any
}

// Decision хранит решение подписчика: продолжить выполнение или заблокировать шаг.
type Decision struct {
	OK      bool
	Block   string
	Message string
	Source  string
}

// Allow возвращает нейтральное решение, которое не меняет ход agent loop.
func Allow() Decision {
	return Decision{OK: true}
}

// Block возвращает решение, которое останавливает блокируемую lifecycle-точку.
func Block(source string, reason string, message string) Decision {
	if reason == "" {
		reason = "blocked"
	}
	return Decision{OK: false, Block: reason, Message: message, Source: source}
}

// TraceWriter — минимальный контракт записи trace-событий.
type TraceWriter interface {
	Write(event string, fields map[string]any) error
}

// TraceRef описывает trace текущей сессии без привязки к concrete type.
type TraceRef interface {
	TraceWriter
	TraceID() string
}

// Subscriber получает lifecycle-события из внутренней шины.
type Subscriber interface {
	Publish(Event) Decision
}

// TraceScoped умеет пересоздать подписчика для дочерней trace-сессии.
type TraceScoped interface {
	WithTrace(TraceRef) Subscriber
}

// CloseSubscriber освобождает ресурсы подписчика перед выходом из run.
type CloseSubscriber interface {
	Close() error
}
