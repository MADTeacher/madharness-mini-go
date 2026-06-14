package trace

import "fmt"

type concurrencySummary struct {
	HasPlans                bool
	MaxParallelTools        int
	MaxParallelSubagents    int
	ParallelReadBatches     int
	ParallelDelegateBatches int
	BarrierGroups           int
}

func collectConcurrencySummary(events []map[string]any) concurrencySummary {
	summary := concurrencySummary{}
	for _, event := range events {
		if event["event"] != "tool_execution_plan" {
			continue
		}
		summary.HasPlans = true
		if maxParallel := intFromAny(event["max_parallel_tool_calls"]); maxParallel > summary.MaxParallelTools {
			summary.MaxParallelTools = maxParallel
		}
		if maxParallel := intFromAny(event["max_parallel_subagents"]); maxParallel > summary.MaxParallelSubagents {
			summary.MaxParallelSubagents = maxParallel
		}
		for _, group := range planGroups(event["groups"]) {
			if boolFromAny(group["parallel"]) {
				switch group["kind"] {
				case "delegate":
					summary.ParallelDelegateBatches++
				default:
					summary.ParallelReadBatches++
				}
			} else {
				summary.BarrierGroups++
			}
		}
	}
	return summary
}

func (s concurrencySummary) String() string {
	return fmt.Sprintf(
		"concurrency: max tool calls %d; max subagents %d; parallel read batches %d; parallel delegate batches %d; barrier groups %d",
		s.MaxParallelTools,
		s.MaxParallelSubagents,
		s.ParallelReadBatches,
		s.ParallelDelegateBatches,
		s.BarrierGroups,
	)
}

func planGroups(raw any) []map[string]any {
	switch groups := raw.(type) {
	case []map[string]any:
		return groups
	case []any:
		out := make([]map[string]any, 0, len(groups))
		for _, item := range groups {
			if group, ok := item.(map[string]any); ok {
				out = append(out, group)
			}
		}
		return out
	default:
		return nil
	}
}

func boolFromAny(value any) bool {
	if parsed, ok := value.(bool); ok {
		return parsed
	}
	return false
}
