package agent

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/subagents"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func runSubagent(
	cfg *config.Config,
	client chatClient,
	parentTrace *trace.Trace,
	subagent subagents.Subagent,
	args map[string]any,
) tools.Observation {
	task := strings.TrimSpace(tools.StringArg(args, "task", ""))
	if task == "" {
		return tools.Fail("delegate_task", "empty subagent task", map[string]any{"subagent": subagent.Name})
	}
	requestedProfile := strings.TrimSpace(tools.StringArg(args, "profile", ""))
	allowedTools, err := subagents.EffectiveTools(subagent, requestedProfile)
	if err != nil {
		return tools.Fail("delegate_task", err.Error(), map[string]any{"subagent": subagent.Name})
	}
	subTrace, err := parentTrace.Child("subagent", "subagent-"+subagent.Name)
	if err != nil {
		return tools.Fail("delegate_task", err.Error(), map[string]any{"subagent": subagent.Name})
	}
	tracePath := subagents.TracePathForObservation(subTrace.Path, cfg.CWD)
	_ = parentTrace.Write("subagent_started", map[string]any{
		"name":       subagent.Name,
		"profile":    effectiveProfile(subagent, requestedProfile),
		"trace_id":   subTrace.ID,
		"trace_path": tracePath,
	})

	result, err := runSubagentLoop(cfg, client, subTrace, subagent, args, task, allowedTools)
	if err != nil {
		_ = parentTrace.Write("subagent_failed", map[string]any{
			"name":       subagent.Name,
			"trace_id":   subTrace.ID,
			"trace_path": tracePath,
			"error":      err.Error(),
		})
		return tools.Fail("delegate_task", "subagent failed: "+err.Error(), map[string]any{
			"subagent":            subagent.Name,
			"subagent_trace_id":   subTrace.ID,
			"subagent_trace_path": tracePath,
		})
	}
	summary := subagents.SummarizeTrace(subTrace.Path)
	_ = parentTrace.Write("subagent_finished", map[string]any{
		"name":          subagent.Name,
		"status":        result.Status,
		"trace_id":      subTrace.ID,
		"trace_path":    tracePath,
		"turns":         result.Turns,
		"trace_summary": summary.Text,
	})
	base := map[string]any{
		"subagent":            subagent.Name,
		"status":              result.Status,
		"turns":               result.Turns,
		"subagent_trace_id":   subTrace.ID,
		"subagent_trace_path": tracePath,
		"trace_summary":       summary.Text,
		"changed_files":       summary.ChangedFiles,
	}
	if result.Status == "done" {
		base["answer"] = result.Result
		return tools.OK("delegate_task", fmt.Sprintf("%s finished in %d turns", subagent.Name, result.Turns), base)
	}
	if result.Status == "needs_user_input" {
		for key, value := range result.Observation {
			if key == "question" || key == "options" || key == "reason" {
				base[key] = value
			}
		}
		return tools.OK("delegate_task", subagent.Name+" needs user input", base)
	}
	return tools.Fail("delegate_task", result.Result, base)
}

func runSubagentLoop(
	cfg *config.Config,
	client chatClient,
	tr *trace.Trace,
	subagent subagents.Subagent,
	args map[string]any,
	task string,
	allowedTools []string,
) (loopResult, error) {
	contextMaxTokens := subagent.ContextMaxTokens
	if contextMaxTokens == 0 {
		contextMaxTokens = cfg.Data.SubagentContextMaxTokens
	}
	if contextMaxTokens == 0 {
		contextMaxTokens = cfg.Data.ContextMaxTokens
	}
	delegatedTask := task
	parentContext := strings.TrimSpace(tools.StringArg(args, "context", ""))
	if parentContext != "" {
		delegatedTask += "\n\n# Parent context\n\n" + parentContext
	}
	context, err := agentcontext.BaseContextWithOptions(cfg, delegatedTask, agentcontext.Options{
		MaxTokens:       contextMaxTokens,
		KeepRecentTurns: cfg.Data.ContextKeepRecentTurns,
	})
	if err != nil {
		return loopResult{}, err
	}
	context.AddFragment(agentcontext.Fragment{
		ID:        "subagent:" + subagent.Name,
		Source:    subagent.Location,
		Text:      renderSubagentPrompt(subagent, allowedTools),
		Priority:  5,
		Placement: "system",
	})
	registry, err := tools.NewRegistryWithOptions(cfg, tools.RegistryOptions{
		Trace:                 tr,
		AllowedTools:          allowedTools,
		WritableSuffixes:      subagentWritableSuffixes(subagent),
		WriteScopeDescription: subagentWriteScopeDescription(subagent),
	}, builtin.Provider{}, subagents.AskUserProvider{})
	if err != nil {
		return loopResult{}, err
	}
	defer registry.Close()
	maxTurns := subagent.MaxTurns
	if maxTurns == 0 {
		maxTurns = cfg.Data.SubagentMaxTurns
	}
	return runModelLoop(client, tr, context, registry, maxTurns, loopOptions{StopOnUserInput: true})
}

func renderSubagentPrompt(subagent subagents.Subagent, allowedTools []string) string {
	toolsText := "none"
	if len(allowedTools) > 0 {
		toolsText = strings.Join(allowedTools, ", ")
	}
	return "# Subagent: " + subagent.Name + "\n\n" +
		"description: " + subagent.Description + "\n" +
		"profile: " + subagent.Profile + "\n" +
		"tools: " + toolsText + "\n" +
		"source: " + subagent.Location + "\n\n" +
		subagent.Prompt
}

func effectiveProfile(subagent subagents.Subagent, requested string) string {
	if requested != "" {
		return requested
	}
	return subagent.Profile
}

func subagentWritableSuffixes(subagent subagents.Subagent) []string {
	if subagent.Name == "planner" {
		return []string{".md"}
	}
	return nil
}

func subagentWriteScopeDescription(subagent subagents.Subagent) string {
	if subagent.Name == "planner" {
		return "planner may write only Markdown plan files (.md)"
	}
	return ""
}
