// Package turnexec планирует и выполняет tool calls одного assistant-turn.
package turnexec

import "github.com/MADTeacher/madharness-mini-go/internal/tools"

// Task описывает один tool call после разбора имени и аргументов.
type Task struct {
	Index       int
	Call        map[string]any
	Name        string
	Args        map[string]any
	Effect      tools.Effect
	Runnable    bool
	Observation tools.Observation
	Followups   []map[string]any
}

// Result хранит итог handler-а или заранее подготовленный отказ.
type Result struct {
	Task        Task
	Observation tools.Observation
	Followups   []map[string]any
}

// Handler выполняет runnable task и возвращает observation с follow-up messages.
type Handler func(Task) (tools.Observation, []map[string]any)

func (t Task) readOnly() bool {
	return t.Runnable && tools.NormalizeEffect(t.Effect) == tools.EffectRead
}

func (t Task) mcp() bool {
	return t.Runnable && tools.NormalizeEffect(t.Effect) == tools.EffectMCP
}

func (t Task) delegate() bool {
	return t.Runnable && tools.NormalizeEffect(t.Effect) == tools.EffectDelegate
}
