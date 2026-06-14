package trace

import "fmt"

type spanSummary struct {
	Total    int
	Open     int
	MaxDepth int
}

func collectSpanSummary(events []map[string]any) spanSummary {
	started := map[string]string{}
	ended := map[string]bool{}
	for _, event := range events {
		if event["event"] != "span_start" {
			if event["event"] == "span_end" {
				id := stringValue(event["span_id"])
				if id != "" {
					ended[id] = true
				}
			}
			continue
		}
		id := stringValue(event["span_id"])
		if id == "" {
			continue
		}
		started[id] = stringValue(event["parent_span_id"])
	}
	summary := spanSummary{Total: len(started)}
	for id := range started {
		if !ended[id] {
			summary.Open++
		}
		if depth := spanDepth(id, started); depth > summary.MaxDepth {
			summary.MaxDepth = depth
		}
	}
	return summary
}

func (s spanSummary) String() string {
	return fmt.Sprintf("spans: total %d; open %d; max depth %d", s.Total, s.Open, s.MaxDepth)
}

func spanDepth(id string, parents map[string]string) int {
	depth := 1
	seen := map[string]bool{id: true}
	for parent := parents[id]; parent != ""; parent = parents[parent] {
		if seen[parent] {
			break
		}
		seen[parent] = true
		depth++
		if _, ok := parents[parent]; !ok {
			break
		}
	}
	return depth
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
