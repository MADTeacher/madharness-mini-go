package agentcontext

import "strings"

func renderMessages(userTask string, fragments []Fragment, entries []HistoryEntry) []map[string]any {
	messages := []map[string]any{}
	systemParts := []string{}
	for _, fragment := range fragments {
		if fragment.Placement == "system" && strings.TrimSpace(fragment.Text) != "" {
			systemParts = append(systemParts, strings.TrimRight(fragment.Text, "\r\n\t "))
		}
	}
	if len(systemParts) > 0 {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": strings.Join(systemParts, "\n\n"),
		})
	}
	for _, fragment := range fragments {
		if fragment.Placement != "user" || strings.TrimSpace(fragment.Text) == "" {
			continue
		}
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": strings.TrimRight(fragment.Text, "\r\n\t "),
		})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userTask})
	for _, entry := range entries {
		messages = append(messages, entry.renderedMessages()...)
	}
	return messages
}
