package trace

import "fmt"

type concurrencySummary struct {
	HasPlans            bool
	MaxParallel         int
	ParallelReadBatches int
	BarrierGroups       int
}

func collectConcurrencySummary(events []map[string]any) concurrencySummary {
	summary := concurrencySummary{}
	for _, event := range events {
		if event["event"] != "tool_execution_plan" {
			continue
		}
		summary.HasPlans = true
		if maxParallel := intFromAny(event["max_parallel_tool_calls"]); maxParallel > summary.MaxParallel {
			summary.MaxParallel = maxParallel
		}
		for _, group := range planGroups(event["groups"]) {
			if boolFromAny(group["parallel"]) {
				summary.ParallelReadBatches++
			} else {
				summary.BarrierGroups++
			}
		}
	}
	return summary
}

func (s concurrencySummary) String() string {
	return fmt.Sprintf(
		"concurrency: max parallel %d; parallel read batches %d; barrier groups %d",
		s.MaxParallel,
		s.ParallelReadBatches,
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
