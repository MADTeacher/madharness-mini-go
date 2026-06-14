package tools

import (
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
)

// SafePathForTool проверяет путь для model-invoked tool и при необходимости
// спрашивает approval для эскалируемого protected path.
func (c *Context) SafePathForTool(tool string, raw string, action string) (string, error) {
	decision := c.Policy.SafePathDecision(raw)
	if decision.Allowed {
		return decision.Path, nil
	}
	if !decision.Escalatable {
		return "", fmt.Errorf("%s", decision.Reason)
	}
	if c.requestApproval(approval.Request{
		Tool:    tool,
		Action:  action,
		Subject: raw,
		Code:    decision.Code,
		Reason:  decision.Reason,
		Details: map[string]any{"path": raw},
	}).Approved {
		return decision.Path, nil
	}
	return "", fmt.Errorf("%s", decision.Reason)
}

// ApprovalForTool возвращает решение для произвольного эскалируемого действия.
func (c *Context) ApprovalForTool(request approval.Request) approval.Decision {
	return c.requestApproval(request)
}

func (c *Context) requestApproval(request approval.Request) approval.Decision {
	if c == nil || c.Approval == nil {
		return approval.Decision{Approved: false, Source: approval.SourceConfig, Reason: request.Reason}
	}
	return c.Approval.Decide(request)
}
