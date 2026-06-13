package agentcontext

// HistoryEntry хранит один атомарный элемент истории диалога.
//
// Если assistant вызвал инструменты, его сообщение и все role=tool ответы
// удаляются или сохраняются вместе, чтобы следующий запрос не получил
// незакрытую пару tool calls.
type HistoryEntry struct {
	Kind                string
	Messages            []map[string]any
	ExpectedToolCallIDs map[string]bool
	SeenToolCallIDs     map[string]bool
	PendingFollowups    []map[string]any
}

func newHistoryEntry(kind string, messages []map[string]any, expected map[string]bool) HistoryEntry {
	if expected == nil {
		expected = map[string]bool{}
	}
	return HistoryEntry{
		Kind:                kind,
		Messages:            messages,
		ExpectedToolCallIDs: expected,
		SeenToolCallIDs:     map[string]bool{},
		PendingFollowups:    []map[string]any{},
	}
}

func (entry HistoryEntry) renderedMessages() []map[string]any {
	rendered := copyMessages(entry.Messages)
	if toolCallsClosed(entry.ExpectedToolCallIDs, entry.SeenToolCallIDs) {
		rendered = append(rendered, copyMessages(entry.PendingFollowups)...)
	}
	return rendered
}

func toolCallsClosed(expected map[string]bool, seen map[string]bool) bool {
	for id := range expected {
		if !seen[id] {
			return false
		}
	}
	return true
}
