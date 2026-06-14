package approval

import (
	"errors"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/events"
)

func TestManagerAsksPrompterAndApproves(t *testing.T) {
	bus := &recordingBus{}
	manager := &Manager{
		Mode:   ModeAsk,
		Kind:   "run",
		Events: bus,
		Prompter: StaticPrompter{Decision: Decision{
			Approved: true,
			Source:   SourceCLI,
			Message:  "yes",
		}},
	}

	decision := manager.Decide(testRequest())
	if !decision.Approved || decision.Source != SourceCLI {
		t.Fatalf("decision = %#v", decision)
	}
	if !bus.has("approval_request") || !bus.has("approval_decision") {
		t.Fatalf("events = %#v", bus.names())
	}
}

func TestManagerDeniesFromPrompter(t *testing.T) {
	manager := &Manager{
		Mode:     ModeAsk,
		Prompter: StaticPrompter{Decision: Decision{Approved: false, Source: SourceCLI, Message: "no"}},
	}

	decision := manager.Decide(testRequest())
	if decision.Approved || decision.Source != SourceCLI || decision.Reason == "" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestManagerDeniesWithoutPrompter(t *testing.T) {
	manager := &Manager{Mode: ModeAsk}

	decision := manager.Decide(testRequest())
	if decision.Approved || decision.Source != SourceConfig {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestManagerYoloAutoApproves(t *testing.T) {
	manager := &Manager{Mode: ModeDeny, Yolo: true}

	decision := manager.Decide(testRequest())
	if !decision.Approved || decision.Source != SourceYOLO {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestManagerHookBlockWinsBeforePrompt(t *testing.T) {
	bus := &recordingBus{blockRequest: true}
	manager := &Manager{
		Mode:     ModeAsk,
		Events:   bus,
		Prompter: StaticPrompter{Decision: Decision{Approved: true, Source: SourceCLI}},
	}

	decision := manager.Decide(testRequest())
	if decision.Approved || decision.Source != "policy-hook" || decision.Reason != "no" {
		t.Fatalf("decision = %#v", decision)
	}
	if !bus.has("approval_decision") {
		t.Fatalf("events = %#v", bus.names())
	}
}

func TestManagerPrompterErrorDenies(t *testing.T) {
	manager := &Manager{
		Mode:     ModeAsk,
		Prompter: StaticPrompter{Err: errors.New("closed")},
	}

	decision := manager.Decide(testRequest())
	if decision.Approved || decision.Source != SourceCLI || decision.Message != "closed" {
		t.Fatalf("decision = %#v", decision)
	}
}

type recordingBus struct {
	events       []events.Event
	blockRequest bool
}

func (b *recordingBus) Publish(event events.Event) events.Decision {
	b.events = append(b.events, event)
	if b.blockRequest && event.Name == "approval_request" {
		return events.Block("policy-hook", "no", "blocked")
	}
	return events.Allow()
}

func (b *recordingBus) has(name string) bool {
	for _, event := range b.events {
		if event.Name == name {
			return true
		}
	}
	return false
}

func (b *recordingBus) names() []string {
	out := []string{}
	for _, event := range b.events {
		out = append(out, event.Name)
	}
	return out
}

func testRequest() Request {
	return Request{Tool: "run_shell", Action: "run_shell", Subject: "curl --version", Code: "risky", Reason: "risky shell command denied"}
}
