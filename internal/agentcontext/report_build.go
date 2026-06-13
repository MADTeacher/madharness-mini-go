package agentcontext

func (m *Manager) buildReport(
	fragments []Fragment,
	entries []HistoryEntry,
	entryIndexes []int,
	initialTokens int,
	currentEstimate map[string]int,
	truncated bool,
	hardLimitExceeded bool,
	clipLimitChars int,
	clippedToolMessages []map[string]any,
	droppedEntries []map[string]any,
) map[string]any {
	fragmentItems := make([]map[string]any, 0, len(fragments))
	for _, fragment := range fragments {
		fragmentItems = append(fragmentItems, fragmentReport(fragment))
	}
	includedEntries := make([]map[string]any, 0, len(entries))
	for index, entry := range entries {
		includedEntries = append(includedEntries, historyEntryReport(entry, entryIndexes[index]))
	}
	return map[string]any{
		"max_tokens":                      m.maxTokens,
		"initial_request_tokens_estimate": initialTokens,
		"messages_tokens_estimate":        currentEstimate["messages_tokens_estimate"],
		"tools_tokens_estimate":           currentEstimate["tools_tokens_estimate"],
		"request_tokens_estimate":         currentEstimate["request_tokens_estimate"],
		"over_budget":                     m.maxTokens > 0 && initialTokens > m.maxTokens,
		"truncated":                       truncated,
		"hard_limit_exceeded":             hardLimitExceeded,
		"fragments":                       fragmentItems,
		"history": map[string]any{
			"total_entries":         len(m.history),
			"rendered_entries":      len(entries),
			"keep_recent_turns":     m.keepRecentTurns,
			"clip_limit_chars":      clipLimitChars,
			"clipped_tool_messages": clippedToolMessages,
			"dropped_entries":       droppedEntries,
			"included_entries":      includedEntries,
		},
	}
}
