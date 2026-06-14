package events

// TraceSubscriber пишет lifecycle-события в существующий JSONL trace.
type TraceSubscriber struct {
	trace TraceRef
}

// NewTraceSubscriber создаёт подписчика, который сохраняет старые имена trace events.
func NewTraceSubscriber(trace TraceRef) *TraceSubscriber {
	return &TraceSubscriber{trace: trace}
}

// Publish пишет событие в trace, если у него задано trace-имя.
func (s *TraceSubscriber) Publish(event Event) Decision {
	if s == nil || s.trace == nil || event.TraceName == "" {
		return Allow()
	}
	_ = s.trace.Write(event.TraceName, event.TraceData)
	return Allow()
}

// WithTrace возвращает такой же trace-подписчик для дочерней trace.
func (s *TraceSubscriber) WithTrace(trace TraceRef) Subscriber {
	return NewTraceSubscriber(trace)
}
