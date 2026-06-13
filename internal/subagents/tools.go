package subagents

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// Runner запускает выбранного субагента из handler-а delegate_task.
type Runner func(*tools.Context, Subagent, map[string]any) tools.Observation

// OrchestratorProvider добавляет parent agent инструмент delegate_task.
type OrchestratorProvider struct {
	Index  Index
	Runner Runner
}

// Specs возвращает delegate_task, если каталог ролей не пуст.
func (p OrchestratorProvider) Specs(ctx *tools.Context) ([]tools.Spec, error) {
	_ = ctx
	if len(p.Index.Subagents) == 0 {
		return nil, nil
	}
	return []tools.Spec{{
		Name:        "delegate_task",
		Description: delegateDescription(p.Index),
		Parameters: tools.Obj(map[string]any{
			"subagent": map[string]any{
				"type":        "string",
				"description": "Built-in role or project-local subagent name.",
				"enum":        p.Index.Names(),
			},
			"task": tools.StrParam("", "Small self-contained task for the subagent.", true),
			"context": tools.StrParam(
				"",
				"Short parent context, constraints, or decisions the subagent should know.",
				false,
			),
			"profile": map[string]any{
				"type":        "string",
				"description": "Optional requested profile. read-only can downgrade writable subagents; writable cannot upgrade read-only subagents.",
				"enum":        []string{"read-only", "writable"},
			},
		}, []string{"subagent", "task"}),
		Handler: p.delegate,
	}}, nil
}

func (p OrchestratorProvider) delegate(ctx *tools.Context, args map[string]any) tools.Observation {
	name := strings.TrimSpace(tools.StringArg(args, "subagent", ""))
	subagent, ok := p.Index.Subagents[name]
	if !ok {
		return tools.Fail("delegate_task", "unknown subagent: "+name)
	}
	if p.Runner == nil {
		return tools.Fail("delegate_task", "subagent runner is not configured", map[string]any{"subagent": name})
	}
	return p.Runner(ctx, subagent, args)
}

// AskUserProvider добавляет субагенту ask_user, если он есть в allow-list.
type AskUserProvider struct{}

// Specs возвращает ask_user; registry сам применит allow-list роли.
func (AskUserProvider) Specs(ctx *tools.Context) ([]tools.Spec, error) {
	_ = ctx
	return []tools.Spec{{
		Name:        "ask_user",
		Description: "Ask the parent agent to stop this delegation and ask the user a concise question.",
		Parameters: tools.Obj(map[string]any{
			"question": tools.StrParam("", "Short question the parent should ask the user.", true),
			"options": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"default":     []string{},
				"description": "Optional 2-3 short options when choices are clear.",
			},
			"reason": tools.StrParam("", "Why the answer is needed before continuing.", false),
		}, []string{"question"}),
		Handler: askUser,
	}}, nil
}

func askUser(ctx *tools.Context, args map[string]any) tools.Observation {
	_ = ctx
	options := cleanOptions(args["options"])
	obs := tools.OK("ask_user", "user input requested", map[string]any{
		"status":   "needs_user_input",
		"question": strings.TrimSpace(tools.StringArg(args, "question", "")),
		"options":  options,
		"reason":   strings.TrimSpace(tools.StringArg(args, "reason", "")),
	})
	obs["_subagent_stop"] = "needs_user_input"
	return obs
}

// EffectiveTools считает итоговый allow-list tools с учётом безопасного downgrade.
func EffectiveTools(subagent Subagent, requestedProfile string) ([]string, error) {
	requestedProfile = strings.TrimSpace(requestedProfile)
	if requestedProfile != "" && !profileValues[requestedProfile] {
		return nil, fmt.Errorf("invalid requested profile: %s", requestedProfile)
	}
	if requestedProfile == "writable" && subagent.Profile != "writable" {
		return nil, fmt.Errorf("subagent is not writable: %s", subagent.Name)
	}
	toolsList := append([]string{}, subagent.Tools...)
	if requestedProfile == "read-only" {
		denied := map[string]bool{"apply_patch": true, "write_file": true, "run_shell": true}
		filtered := []string{}
		for _, name := range toolsList {
			if !denied[name] {
				filtered = append(filtered, name)
			}
		}
		toolsList = filtered
	}
	return toolsList, nil
}

func delegateDescription(index Index) string {
	lines := []string{
		"Delegate a small task to a built-in or project-local subagent.",
		"The subagent runs in its own context with its own local trace.",
		"Available subagents:",
	}
	for _, name := range index.Names() {
		item := index.Subagents[name]
		lines = append(lines, fmt.Sprintf("- %s (%s): %s", item.Name, item.Profile, item.Description))
	}
	return strings.Join(lines, "\n")
}

func cleanOptions(raw any) []string {
	values := []string{}
	switch items := raw.(type) {
	case []string:
		values = items
	case []any:
		for _, item := range items {
			values = append(values, fmt.Sprint(item))
		}
	}
	out := []string{}
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" {
			out = append(out, text)
		}
		if len(out) == 3 {
			break
		}
	}
	return out
}
