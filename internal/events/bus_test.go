package events

import "testing"

func TestBusPublishesSubscribersInOrderAndStopsOnBlock(t *testing.T) {
	calls := []string{}
	bus := NewBus(
		recordingSubscriber{id: "first", calls: &calls},
		recordingSubscriber{id: "blocker", calls: &calls, block: "nope"},
		recordingSubscriber{id: "third", calls: &calls},
	)

	decision := bus.Publish(Event{Name: "before_tool_call"})

	if decision.OK || decision.Block != "nope" || decision.Source != "blocker" {
		t.Fatalf("decision = %#v", decision)
	}
	if joined := joinCalls(calls); joined != "first,blocker" {
		t.Fatalf("calls = %s", joined)
	}
}

func TestBusWithTraceRebindsTraceScopedSubscribers(t *testing.T) {
	parentTrace := &memoryTrace{id: "parent"}
	childTrace := &memoryTrace{id: "child"}
	bus := NewBus(NewTraceSubscriber(parentTrace))
	child := bus.WithTrace(childTrace)

	child.Publish(Event{TraceName: "session_end", TraceData: map[string]any{"result": "ok"}})

	if len(parentTrace.events) != 0 {
		t.Fatalf("parent trace events = %#v", parentTrace.events)
	}
	if len(childTrace.events) != 1 || childTrace.events[0] != "session_end" {
		t.Fatalf("child trace events = %#v", childTrace.events)
	}
}

func TestBusCloseCallsSubscribers(t *testing.T) {
	closed := 0
	bus := NewBus(closeSubscriber{closed: &closed})

	if err := bus.Close(); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("closed = %d", closed)
	}
}

func TestTraceSubscriberIgnoresEventsWithoutTraceName(t *testing.T) {
	trace := &memoryTrace{id: "trace"}
	subscriber := NewTraceSubscriber(trace)

	subscriber.Publish(Event{Name: "before_tool_call"})

	if len(trace.events) != 0 {
		t.Fatalf("trace events = %#v", trace.events)
	}
}

type recordingSubscriber struct {
	id    string
	calls *[]string
	block string
}

func (s recordingSubscriber) Publish(Event) Decision {
	*s.calls = append(*s.calls, s.id)
	if s.block != "" {
		return Block(s.id, s.block, "")
	}
	return Allow()
}

type closeSubscriber struct {
	closed *int
}

func (s closeSubscriber) Publish(Event) Decision {
	return Allow()
}

func (s closeSubscriber) Close() error {
	*s.closed = *s.closed + 1
	return nil
}

type memoryTrace struct {
	id     string
	events []string
	fields []map[string]any
}

func (t *memoryTrace) TraceID() string {
	return t.id
}

func (t *memoryTrace) Write(event string, fields map[string]any) error {
	t.events = append(t.events, event)
	t.fields = append(t.fields, fields)
	return nil
}

func joinCalls(calls []string) string {
	out := ""
	for index, call := range calls {
		if index > 0 {
			out += ","
		}
		out += call
	}
	return out
}
