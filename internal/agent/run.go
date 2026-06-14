package agent

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/hooks"
	"github.com/MADTeacher/madharness-mini-go/internal/mcp"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/skills"
	"github.com/MADTeacher/madharness-mini-go/internal/subagents"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// RunOptions задаёт режимы одного запуска run.
type RunOptions struct {
	OrchestrationMode    string
	MaxParallelToolCalls int
}

// Run запускает агентский цикл до финального ответа или max_turns.
func Run(task string, cfg *config.Config) (string, string, error) {
	return RunWithOptions(task, cfg, RunOptions{})
}

// RunWithOptions запускает agent loop с CLI-переопределениями одного запуска.
func RunWithOptions(task string, cfg *config.Config, options RunOptions) (string, string, error) {
	return runWithClientOptions(task, cfg, model.New(cfg), options)
}

func runWithClient(task string, cfg *config.Config, client chatClient) (string, string, error) {
	return runWithClientOptions(task, cfg, client, RunOptions{})
}

func runWithClientOptions(task string, cfg *config.Config, client chatClient, options RunOptions) (string, string, error) {
	tr, err := trace.New(cfg, "run")
	if err != nil {
		return "", "", err
	}
	hookManager, err := hooks.FromConfig(cfg, tr)
	if err != nil {
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		return "", tr.Path, err
	}
	hookManager.Emit("session_start", "run", map[string]any{
		"task_preview": truncateForHook(task, 1000),
		"cwd":          cfg.CWD,
	})
	index := skills.Discover(cfg)
	subagentIndex := subagents.Discover(cfg)
	orchestration, err := subagents.ResolveOrchestrationMode(cfg, task, options.OrchestrationMode)
	if err != nil {
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "run", err, nil)
		return "", tr.Path, err
	}
	_ = tr.Write("orchestration_mode", map[string]any{
		"configured":        orchestration.Configured,
		"effective":         orchestration.Effective,
		"source":            orchestration.Source,
		"requested_by_task": orchestration.RequestedByTask,
		"legacy_enabled":    cfg.Data.OrchestrationEnabled,
	})
	_ = tr.Write("skills_discovered", map[string]any{
		"count":       len(index.Skills),
		"names":       index.Names(),
		"diagnostics": diagnosticsForTrace(index.Diagnostics, cfg.Root),
	})
	_ = tr.Write("subagents_discovered", map[string]any{
		"count":       len(subagentIndex.Subagents),
		"names":       subagentIndex.Names(),
		"diagnostics": subagentDiagnosticsForTrace(subagentIndex.Diagnostics),
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
		emitSessionError(hookManager, "run", err, nil)
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
	if orchestration.Effective != "off" {
		toolProviders = append(toolProviders, subagents.OrchestratorProvider{
			Index: subagentIndex,
			Runner: func(ctx *tools.Context, subagent subagents.Subagent, args map[string]any) tools.Observation {
				return runSubagent(cfg, client, tr, subagent, args, hookManager)
			},
		})
	}
	if orchestration.Effective != "required" {
		toolProviders = append(toolProviders, &mcp.ToolProvider{})
	}
	registry, err := tools.NewRegistryWithOptions(cfg, tools.RegistryOptions{
		Trace:           tr,
		ResourceTracker: runtime,
		AllowedTools:    subagents.ParentAllowedTools(orchestration.Effective),
	}, toolProviders...)
	if err != nil {
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "run", err, nil)
		return "", tr.Path, err
	}
	defer registry.Close()
	context, err := BaseContext(cfg, task, contextProviders...)
	if err != nil {
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "run", err, nil)
		return "", tr.Path, err
	}
	if orchestration.Effective == "required" {
		context.AddFragment(subagents.RequiredFragment())
	}
	for _, name := range selection.Names {
		obs := runtime.Activate(name, "explicit")
		if obs["ok"] != true {
			err := fmt.Errorf("%v", obs["summary"])
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			emitSessionError(hookManager, "run", err, nil)
			return "", tr.Path, err
		}
		applyHiddenObservationEffects(context, tr, obs)
	}
	result, err := runModelLoop(client, tr, context, registry, cfg.Data.MaxTurns, loopOptions{
		Hooks:                hookManager,
		Kind:                 "run",
		MaxParallelToolCalls: resolvedMaxParallelToolCalls(cfg, options),
	})
	return result.Result, tr.Path, err
}

func diagnosticsForTrace(diagnostics []skills.Diagnostic, root string) []map[string]string {
	out := make([]map[string]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.AsMap(root))
	}
	return out
}

func subagentDiagnosticsForTrace(diagnostics []subagents.Diagnostic) []map[string]string {
	out := make([]map[string]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.AsMap())
	}
	return out
}
