// Package agentcontext собирает сообщения, которые агент отправит модели.
package agentcontext

// Fragment описывает отдельный кусок контекста для следующего запроса.
//
// Закреплённые фрагменты вроде system prompt и AGENTS.md живут отдельно от
// истории: бюджет может сжимать старые ответы инструментов, но не должен
// случайно удалить правила проекта или исходную задачу.
type Fragment struct {
	ID        string
	Source    string
	Text      string
	Priority  int
	Placement string
	Transient bool
}

// State показывает provider-ам только безопасное состояние слоя контекста.
type State struct {
	UserTask        string
	FragmentsCount  int
	HistoryEntries  int
	MaxTokens       int
	KeepRecentTurns int
}

// Provider добавляет временные или постоянные фрагменты перед model call.
type Provider interface {
	Collect(State) []Fragment
}

func normalizeFragment(fragment Fragment) Fragment {
	if fragment.Placement == "" {
		fragment.Placement = "system"
	}
	if fragment.Priority == 0 && fragment.ID != "system" {
		fragment.Priority = 100
	}
	return fragment
}
