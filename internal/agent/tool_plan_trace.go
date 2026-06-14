package agent

import (
	"github.com/MADTeacher/madharness-mini-go/internal/agent/turnexec"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func writeToolExecutionPlan(tr *trace.Trace, turn int, maxParallelTools int, maxParallelSubagents int, groups []turnexec.Group) {
	if tr == nil {
		return
	}
	items := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		items = append(items, map[string]any{
			"parallel": group.Parallel,
			"kind":     group.Kind,
			"count":    len(group.Tasks),
			"tools":    taskNames(group.Tasks),
			"effects":  taskEffects(group.Tasks),
		})
	}
	_ = tr.Write("tool_execution_plan", map[string]any{
		"turn":                    turn,
		"max_parallel_tool_calls": maxParallelTools,
		"max_parallel_subagents":  maxParallelSubagents,
		"groups":                  items,
	})
}

func taskNames(tasks []turnexec.Task) []string {
	names := make([]string, 0, len(tasks))
	for _, task := range tasks {
		names = append(names, task.Name)
	}
	return names
}

func taskEffects(tasks []turnexec.Task) []string {
	effects := make([]string, 0, len(tasks))
	for _, task := range tasks {
		effects = append(effects, string(task.Effect))
	}
	return effects
}
