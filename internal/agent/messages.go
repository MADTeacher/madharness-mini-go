// Package agent содержит режимы ask/run и цикл model/tool calls.
package agent

import (
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/prompt"
)

// BaseMessages собирает стартовую историю: system prompt и задачу пользователя.
func BaseMessages(cfg *config.Config, task string) ([]map[string]any, error) {
	system, err := prompt.Load("system")
	if err != nil {
		return nil, err
	}
	_ = cfg
	return []map[string]any{
		{"role": "system", "content": system},
		{"role": "user", "content": task},
	}, nil
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
