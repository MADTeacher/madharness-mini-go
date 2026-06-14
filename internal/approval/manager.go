package approval

import (
	"github.com/MADTeacher/madharness-mini-go/internal/events"
	tracepkg "github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Publisher — минимальный контракт event bus для approval lifecycle-событий.
type Publisher interface {
	Publish(events.Event) events.Decision
}

// Manager соединяет policy-denial, hooks, YOLO и пользовательский prompt.
type Manager struct {
	Mode     string
	Yolo     bool
	Kind     string
	Events   Publisher
	Trace    any
	SpanID   string
	Prompter Prompter
}

// Decide возвращает итог для одного эскалируемого запроса.
func (m *Manager) Decide(request Request) Decision {
	request = normalizeRequest(request)
	if m == nil {
		return deny(SourceConfig, request.Reason, "approval manager is not configured")
	}
	spanID := m.SpanID
	span := m.startApprovalSpan(request)
	if span != nil {
		spanID = span.ID()
	}
	if decision := m.publishRequest(request, spanID); !decision.OK {
		result := deny(sourceOr(decision.Source, SourceHook), blockOr(decision.Block, request.Reason), decision.Message)
		m.publishDecision(request, result, spanID)
		endApprovalSpan(span, result)
		return result
	}
	if m.Yolo {
		result := Decision{Approved: true, Source: SourceYOLO, Reason: request.Reason, Message: "auto-approved by YOLO mode"}
		m.publishDecision(request, result, spanID)
		endApprovalSpan(span, result)
		return result
	}
	if m.mode() == ModeAsk {
		result := m.ask(request)
		m.publishDecision(request, result, spanID)
		endApprovalSpan(span, result)
		return result
	}
	result := deny(SourceConfig, request.Reason, "approval mode is deny")
	m.publishDecision(request, result, spanID)
	endApprovalSpan(span, result)
	return result
}

func (m *Manager) publishRequest(request Request, spanID string) events.Decision {
	if m.Events == nil {
		return events.Allow()
	}
	data := request.data()
	return m.Events.Publish(events.Event{
		Name:      "approval_request",
		Kind:      m.kind(),
		SpanID:    spanID,
		HookData:  data,
		TraceName: "approval_request",
		TraceData: data,
	})
}

func (m *Manager) publishDecision(request Request, decision Decision, spanID string) {
	if m.Events == nil {
		return
	}
	data := decision.data(request)
	_ = m.Events.Publish(events.Event{
		Name:      "approval_decision",
		Kind:      m.kind(),
		SpanID:    spanID,
		HookData:  data,
		TraceName: "approval_decision",
		TraceData: data,
	})
}

func (m *Manager) startApprovalSpan(request Request) *tracepkg.Span {
	starter, ok := m.Trace.(interface {
		StartSpan(string, string, map[string]any) *tracepkg.Span
	})
	if !ok {
		return nil
	}
	return starter.StartSpan("approval", m.SpanID, map[string]any{
		"action": request.Action,
		"code":   request.Code,
		"reason": request.Reason,
	})
}

func endApprovalSpan(span *tracepkg.Span, decision Decision) {
	if span == nil {
		return
	}
	status := "denied"
	if decision.Approved {
		status = "ok"
	}
	span.End(status, map[string]any{"source": decision.Source, "reason": decision.Reason})
}

func (m *Manager) ask(request Request) Decision {
	if m.Prompter == nil {
		return deny(SourceConfig, request.Reason, "approval prompter is not configured")
	}
	decision, err := m.Prompter.Prompt(request)
	if err != nil {
		return deny(SourceCLI, request.Reason, clipped(err.Error(), 1000))
	}
	if decision.Source == "" {
		decision.Source = SourceCLI
	}
	if decision.Reason == "" {
		decision.Reason = request.Reason
	}
	return decision
}

func clipped(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

func (m *Manager) mode() string {
	if m == nil || m.Mode == "" {
		return ModeDeny
	}
	if m.Mode == ModeAsk {
		return ModeAsk
	}
	return ModeDeny
}

func (m *Manager) kind() string {
	if m == nil || m.Kind == "" {
		return "run"
	}
	return m.Kind
}

func normalizeRequest(request Request) Request {
	if request.Action == "" {
		request.Action = "tool_call"
	}
	if request.Code == "" {
		request.Code = "policy_denied"
	}
	if request.Reason == "" {
		request.Reason = "policy denied"
	}
	if request.Details == nil {
		request.Details = map[string]any{}
	}
	return request
}

func deny(source string, reason string, message string) Decision {
	if reason == "" {
		reason = "approval denied"
	}
	return Decision{Approved: false, Source: sourceOr(source, SourceConfig), Reason: reason, Message: message}
}

func sourceOr(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func blockOr(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
