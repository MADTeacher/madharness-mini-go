package agentcontext

func (m *Manager) dropOldEntriesUntilBudget(
	fragments []Fragment,
	entries *[]HistoryEntry,
	entryIndexes *[]int,
	tools []map[string]any,
	keepRecentTurns int,
	forced bool,
) []map[string]any {
	dropped := []map[string]any{}
	messages := renderMessages(m.userTask, fragments, *entries)
	protectedStart := maxInt(len(*entries)-keepRecentTurns, 0)
	for len(*entries) > 0 && estimateRequestTokens(messages, tools)["request_tokens_estimate"] > m.maxTokens {
		removable := -1
		for index := 0; index < protectedStart; index++ {
			removable = index
			break
		}
		if removable < 0 {
			break
		}
		report := historyEntryReport((*entries)[removable], (*entryIndexes)[removable])
		if forced {
			report["forced"] = true
		}
		dropped = append(dropped, report)
		*entries = append((*entries)[:removable], (*entries)[removable+1:]...)
		*entryIndexes = append((*entryIndexes)[:removable], (*entryIndexes)[removable+1:]...)
		protectedStart = maxInt(len(*entries)-keepRecentTurns, 0)
		messages = renderMessages(m.userTask, fragments, *entries)
	}
	return dropped
}
