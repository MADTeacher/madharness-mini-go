package agentcontext

import (
	"fmt"
	"sort"
)

const (
	DefaultMaxTokens       = 60000
	DefaultKeepRecentTurns = 3
)

// Options задаёт бюджет и provider-ы для одного запуска ask/run.
type Options struct {
	MaxTokens       int
	KeepRecentTurns int
	Providers       []Provider
}

// Manager хранит контекст одного ask/run и собирает Chat Completions messages.
type Manager struct {
	userTask        string
	maxTokens       int
	keepRecentTurns int
	providers       []Provider
	fragments       []Fragment
	history         []HistoryEntry
	lastStats       map[string]any
	lastReport      map[string]any
}

// NewManager создаёт пустой слой контекста для пользовательской задачи.
func NewManager(userTask string, options Options) *Manager {
	useDefaults := options.MaxTokens == 0 && options.KeepRecentTurns == 0 && options.Providers == nil
	maxTokens := options.MaxTokens
	if useDefaults {
		maxTokens = DefaultMaxTokens
	}
	if maxTokens < 0 {
		maxTokens = 0
	}
	keepRecentTurns := options.KeepRecentTurns
	if useDefaults {
		keepRecentTurns = DefaultKeepRecentTurns
	}
	if keepRecentTurns < 0 {
		keepRecentTurns = 0
	}
	return &Manager{
		userTask:        userTask,
		maxTokens:       maxTokens,
		keepRecentTurns: keepRecentTurns,
		providers:       append([]Provider{}, options.Providers...),
	}
}

// AddFragment добавляет или заменяет фрагмент по id.
func (m *Manager) AddFragment(fragment Fragment) {
	fragment = normalizeFragment(fragment)
	next := make([]Fragment, 0, len(m.fragments)+1)
	for _, item := range m.fragments {
		if item.ID != fragment.ID {
			next = append(next, item)
		}
	}
	m.fragments = append(next, fragment)
	m.resetCache()
}

// RecordAssistant запоминает ответ модели как следующий элемент истории.
func (m *Manager) RecordAssistant(message map[string]any) {
	stored := sanitizeAssistantMessage(message)
	expected := map[string]bool{}
	for _, item := range asSlice(stored["tool_calls"]) {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := call["id"].(string); ok && id != "" {
			expected[id] = true
		}
	}
	kind := "assistant"
	if len(expected) > 0 {
		kind = "tool_turn"
	}
	m.history = append(m.history, newHistoryEntry(kind, []map[string]any{stored}, expected))
	m.resetCache()
}

// RecordToolResult добавляет role=tool и скрытые follow-up сообщения.
func (m *Manager) RecordToolResult(call map[string]any, observation map[string]any, followups []map[string]any) {
	entry := m.lastToolEntry()
	callID, _ := call["id"].(string)
	if callID == "" {
		callID = toolCallName(call, observation)
	}
	content := "{}"
	if raw, err := jsonMarshalString(observation); err == nil {
		content = raw
	}
	entry.Messages = append(entry.Messages, map[string]any{
		"role":         "tool",
		"tool_call_id": callID,
		"content":      content,
	})
	entry.SeenToolCallIDs[callID] = true
	entry.PendingFollowups = append(entry.PendingFollowups, copyMessages(followups)...)
	m.resetCache()
}

// Messages возвращает сообщения для модели с учётом бюджета контекста.
func (m *Manager) Messages(tools []map[string]any) ([]map[string]any, error) {
	fragments := m.collectFragments()
	entries := copyHistory(m.history)
	entryIndexes := make([]int, len(entries))
	for index := range entries {
		entryIndexes[index] = index
	}
	messages := renderMessages(m.userTask, fragments, entries)
	initialEstimate := estimateRequestTokens(messages, tools)
	initialTokens := initialEstimate["request_tokens_estimate"]
	truncated := false
	droppedEntries := []map[string]any{}
	clippedToolMessages := []map[string]any{}
	clipLimitChars := 0

	if m.maxTokens > 0 && initialTokens > m.maxTokens {
		clipLimitChars = maxInt(80, minInt(4000, m.maxTokens*TokenEstimateBytesPerToken/8))
		clippedToolMessages = clipToolMessages(entries, clipLimitChars)
		if len(clippedToolMessages) > 0 {
			truncated = true
			messages = renderMessages(m.userTask, fragments, entries)
		}
	}

	currentEstimate := estimateRequestTokens(messages, tools)
	if m.maxTokens > 0 && currentEstimate["request_tokens_estimate"] > m.maxTokens {
		droppedEntries = m.dropOldEntriesUntilBudget(fragments, &entries, &entryIndexes, tools, m.keepRecentTurns, false)
		messages = renderMessages(m.userTask, fragments, entries)
		truncated = truncated || len(droppedEntries) > 0
		currentEstimate = estimateRequestTokens(messages, tools)
	}

	if m.maxTokens > 0 && currentEstimate["request_tokens_estimate"] > m.maxTokens {
		forced := m.dropOldEntriesUntilBudget(fragments, &entries, &entryIndexes, tools, 0, true)
		droppedEntries = append(droppedEntries, forced...)
		messages = renderMessages(m.userTask, fragments, entries)
		truncated = truncated || len(forced) > 0
		currentEstimate = estimateRequestTokens(messages, tools)
	}

	hardLimitExceeded := m.maxTokens > 0 && currentEstimate["request_tokens_estimate"] > m.maxTokens
	m.lastStats = map[string]any{
		"context_tokens_estimate":  currentEstimate["request_tokens_estimate"],
		"messages_tokens_estimate": currentEstimate["messages_tokens_estimate"],
		"tools_tokens_estimate":    currentEstimate["tools_tokens_estimate"],
		"fragments":                len(fragments),
		"history_entries":          len(m.history),
		"dropped_entries":          len(droppedEntries),
		"truncated":                truncated,
		"hard_limit_exceeded":      hardLimitExceeded,
	}
	m.lastReport = m.buildReport(
		fragments,
		entries,
		entryIndexes,
		initialTokens,
		currentEstimate,
		truncated,
		hardLimitExceeded,
		clipLimitChars,
		clippedToolMessages,
		droppedEntries,
	)
	if hardLimitExceeded {
		return nil, m.budgetError(currentEstimate, clippedToolMessages, droppedEntries)
	}
	return messages, nil
}

// Stats возвращает короткую диагностику последней сборки контекста.
func (m *Manager) Stats() map[string]any {
	if m.lastStats == nil {
		_, _ = m.Messages(nil)
	}
	return copyReport(m.lastStats)
}

// Report описывает последнюю сборку контекста без текстов prompt и tool output.
func (m *Manager) Report() map[string]any {
	if m.lastReport == nil {
		_, _ = m.Messages(nil)
	}
	return copyReport(m.lastReport)
}

func (m *Manager) lastToolEntry() *HistoryEntry {
	if len(m.history) > 0 && m.history[len(m.history)-1].Kind == "tool_turn" {
		return &m.history[len(m.history)-1]
	}
	m.history = append(m.history, newHistoryEntry("tool_turn", nil, nil))
	return &m.history[len(m.history)-1]
}

func (m *Manager) collectFragments() []Fragment {
	state := State{
		UserTask:        m.userTask,
		FragmentsCount:  len(m.fragments),
		HistoryEntries:  len(m.history),
		MaxTokens:       m.maxTokens,
		KeepRecentTurns: m.keepRecentTurns,
	}
	fragments := append([]Fragment{}, m.fragments...)
	for _, provider := range m.providers {
		fragments = append(fragments, provider.Collect(state)...)
	}
	for index := range fragments {
		fragments[index] = normalizeFragment(fragments[index])
	}
	sort.SliceStable(fragments, func(i int, j int) bool {
		left := fragments[i]
		right := fragments[j]
		if left.Placement != right.Placement {
			return left.Placement < right.Placement
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return left.ID < right.ID
	})
	return fragments
}

func (m *Manager) resetCache() {
	m.lastStats = nil
	m.lastReport = nil
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func (m *Manager) budgetError(current map[string]int, clipped []map[string]any, dropped []map[string]any) error {
	return fmt.Errorf(
		"context budget exceeded after truncation: request_tokens_estimate=%d messages_tokens_estimate=%d tools_tokens_estimate=%d max_tokens=%d clipped_tool_messages=%d dropped_entries=%d; adjust context_max_tokens, context_keep_recent_turns, task length, tool output, or enabled tools",
		current["request_tokens_estimate"],
		current["messages_tokens_estimate"],
		current["tools_tokens_estimate"],
		m.maxTokens,
		len(clipped),
		len(dropped),
	)
}
