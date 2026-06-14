package subagents

import (
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
)

var parentOnlyTools = map[string]bool{
	"activate_skill": true,
	"delegate_task":  true,
}

const subagentToolPolicy = "subagents expose only builtin tools and ask_user"

// RuntimeToolEffects возвращает tools, которые registry может выдать субагенту.
func RuntimeToolEffects() map[string]tools.Effect {
	out := map[string]tools.Effect{}
	for _, provider := range []tools.Provider{builtin.Provider{}, AskUserProvider{}} {
		specs, err := provider.Specs(nil)
		if err != nil {
			continue
		}
		for _, spec := range specs {
			out[spec.Name] = tools.NormalizeEffect(spec.Effect)
		}
	}
	return out
}

func validateToolName(name string, location string, diagnostics *[]Diagnostic) {
	if parentOnlyTools[name] {
		*diagnostics = append(*diagnostics, Diagnostic{
			"error",
			location,
			name + " is not allowed inside subagent tools; tool is not exposed to subagents",
		})
		return
	}
	if strings.HasPrefix(name, "mcp__") {
		*diagnostics = append(*diagnostics, Diagnostic{
			"error",
			location,
			name + " is not allowed inside subagent tools; MCP tools are not exposed to subagents; " + subagentToolPolicy,
		})
		return
	}
	if _, ok := RuntimeToolEffects()[name]; !ok {
		*diagnostics = append(*diagnostics, Diagnostic{"error", location, "unknown subagent tool: " + name + "; " + subagentToolPolicy})
	}
}

func toolAllowedInReadOnly(name string, effect tools.Effect) bool {
	if name == "ask_user" {
		return true
	}
	return tools.NormalizeEffect(effect) == tools.EffectRead
}
