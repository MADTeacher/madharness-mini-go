// Package agent содержит режимы ask/run и цикл model/tool calls.
package agent

import (
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

// BaseContext собирает стартовый слой контекста: system prompt, AGENTS.md и задачу.
func BaseContext(cfg *config.Config, task string) (*agentcontext.Manager, error) {
	return agentcontext.BaseContext(cfg, task)
}

// BaseMessages сохраняет старый контракт тестов и внешних учебных патчей.
func BaseMessages(cfg *config.Config, task string) ([]map[string]any, error) {
	context, err := BaseContext(cfg, task)
	if err != nil {
		return nil, err
	}
	return context.Messages(nil)
}

func responseMessage(raw map[string]any) (map[string]any, error) {
	choices, ok := raw["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("LLM API response has no choices")
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("LLM API response choice has invalid shape")
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("LLM API response message has invalid shape")
	}
	return message, nil
}

func messageContent(message map[string]any) string {
	if value, ok := message["content"].(string); ok {
		return value
	}
	return ""
}
