package agent

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/skills"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Run запускает агентский цикл до финального ответа или max_turns.
func Run(task string, cfg *config.Config) (string, string, error) {
	return runWithClient(task, cfg, model.New(cfg))
}

func runWithClient(task string, cfg *config.Config, client chatClient) (string, string, error) {
	tr, err := trace.New(cfg, "run")
	if err != nil {
		return "", "", err
	}
	index := skills.Discover(cfg)
	_ = tr.Write("skills_discovered", map[string]any{
		"count":       len(index.Skills),
		"names":       index.Names(),
		"diagnostics": diagnosticsForTrace(index.Diagnostics, cfg.Root),
	})
	selection := skills.FindExplicitSelection(task, index.NameSet())
	if selection.Present() {
		_ = tr.Write("skills_explicit_selection", map[string]any{
			"names":   selection.Names,
			"unknown": selection.Unknown,
		})
	}
	if len(selection.Unknown) > 0 {
		names := strings.Join(selection.Unknown, ", ")
		err := fmt.Errorf("unknown skill: %s", names)
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		return "", tr.Path, err
	}
	runtime := skills.NewRuntime(cfg, index)
	contextProviders := []agentcontext.Provider{}
	toolProviders := []tools.Provider{builtin.Provider{}}
	if len(selection.Names) > 0 {
		_ = tr.Write("skills_auto_selection_disabled", map[string]any{"reason": "explicit skill marker"})
	} else {
		contextProviders = append(contextProviders, skills.CatalogProvider{Index: index, WorkspaceRoot: cfg.Root})
		toolProviders = append(toolProviders, skills.ToolProvider{Runtime: runtime})
	}
	registry, err := tools.NewRegistryWithOptions(cfg, tools.RegistryOptions{
		Trace:           tr,
		ResourceTracker: runtime,
	}, toolProviders...)
	if err != nil {
		return "", tr.Path, err
	}
	context, err := BaseContext(cfg, task, contextProviders...)
	if err != nil {
		return "", tr.Path, err
	}
	for _, name := range selection.Names {
		obs := runtime.Activate(name, "explicit")
		if obs["ok"] != true {
			err := fmt.Errorf("%v", obs["summary"])
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			return "", tr.Path, err
		}
		applyHiddenObservationEffects(context, tr, obs)
	}
	result, err := runModelLoop(client, tr, context, registry, cfg.Data.MaxTurns)
	return result, tr.Path, err
}

func diagnosticsForTrace(diagnostics []skills.Diagnostic, root string) []map[string]string {
	out := make([]map[string]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.AsMap(root))
	}
	return out
}
