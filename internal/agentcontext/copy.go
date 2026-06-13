package agentcontext

import "encoding/json"

func copyMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		out := map[string]any{}
		for key, item := range value {
			out[key] = item
		}
		return out
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func copyMessages(messages []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		out = append(out, copyMap(message))
	}
	return out
}

func copyHistory(entries []HistoryEntry) []HistoryEntry {
	out := make([]HistoryEntry, 0, len(entries))
	for _, entry := range entries {
		expected := map[string]bool{}
		for id, ok := range entry.ExpectedToolCallIDs {
			expected[id] = ok
		}
		seen := map[string]bool{}
		for id, ok := range entry.SeenToolCallIDs {
			seen[id] = ok
		}
		out = append(out, HistoryEntry{
			Kind:                entry.Kind,
			Messages:            copyMessages(entry.Messages),
			ExpectedToolCallIDs: expected,
			SeenToolCallIDs:     seen,
			PendingFollowups:    copyMessages(entry.PendingFollowups),
		})
	}
	return out
}
