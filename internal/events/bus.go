package events

// Bus публикует lifecycle-событие всем внутренним подписчикам по порядку.
type Bus struct {
	subscribers []Subscriber
}

// NewBus собирает шину из подписчиков, пропуская nil.
func NewBus(subscribers ...Subscriber) *Bus {
	bus := &Bus{}
	for _, subscriber := range subscribers {
		if subscriber != nil {
			bus.subscribers = append(bus.subscribers, subscriber)
		}
	}
	return bus
}

// Publish отправляет событие подписчикам и возвращает первое blocking-решение.
func (b *Bus) Publish(event Event) Decision {
	if b == nil {
		return Allow()
	}
	for _, subscriber := range b.subscribers {
		decision := subscriber.Publish(event)
		if !decision.OK {
			return decision
		}
	}
	return Allow()
}

// WithTrace пересобирает trace-aware подписчиков для дочерней trace.
func (b *Bus) WithTrace(trace TraceRef) *Bus {
	if b == nil {
		return nil
	}
	subscribers := make([]Subscriber, 0, len(b.subscribers))
	for _, subscriber := range b.subscribers {
		if scoped, ok := subscriber.(TraceScoped); ok {
			subscribers = append(subscribers, scoped.WithTrace(trace))
			continue
		}
		subscribers = append(subscribers, subscriber)
	}
	return NewBus(subscribers...)
}

// Close даёт подписчикам завершить отложенную работу.
func (b *Bus) Close() error {
	if b == nil {
		return nil
	}
	var first error
	for _, subscriber := range b.subscribers {
		closer, ok := subscriber.(CloseSubscriber)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
