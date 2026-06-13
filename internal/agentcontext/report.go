package agentcontext

import (
	"encoding/json"
	"sort"
)

func fragmentReport(fragment Fragment) map[string]any {
	return map[string]any{
		"id":        fragment.ID,
		"source":    fragment.Source,
		"placement": fragment.Placement,
		"priority":  fragment.Priority,
		"chars":     len(fragment.Text),
		"transient": fragment.Transient,
		"empty":     len(fragment.Text) == 0,
	}
}

func historyEntryReport(entry HistoryEntry, index int) map[string]any {
	rendered := entry.renderedMessages()
	ids := map[string]bool{}
	for id := range entry.ExpectedToolCallIDs {
		ids[id] = true
	}
	for id := range entry.SeenToolCallIDs {
		ids[id] = true
	}
	toolIDs := make([]string, 0, len(ids))
	for id := range ids {
		toolIDs = append(toolIDs, id)
	}
	sort.Strings(toolIDs)
	roles := make([]string, 0, len(rendered))
	for _, message := range rendered {
		if role, ok := message["role"].(string); ok {
			roles = append(roles, role)
		} else {
			roles = append(roles, "")
		}
	}
	return map[string]any{
		"index":             index,
		"kind":              entry.Kind,
		"messages":          len(entry.Messages),
		"rendered_messages": len(rendered),
		"tokens_estimate":   EstimateTokens(rendered),
		"roles":             roles,
		"tool_call_ids":     toolIDs,
		"pending_followups": len(entry.PendingFollowups),
	}
}

func copyReport(report map[string]any) map[string]any {
	if report == nil {
		return nil
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}
