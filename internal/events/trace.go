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
	data := event.TraceData
	if event.SpanID != "" {
		data = cloneTraceData(event.TraceData)
		data["span_id"] = event.SpanID
	}
	_ = s.trace.Write(event.TraceName, data)
	return Allow()
}

// WithTrace возвращает такой же trace-подписчик для дочерней trace.
func (s *TraceSubscriber) WithTrace(trace TraceRef) Subscriber {
	return NewTraceSubscriber(trace)
}

func cloneTraceData(data map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range data {
		out[key] = value
	}
	return out
}
