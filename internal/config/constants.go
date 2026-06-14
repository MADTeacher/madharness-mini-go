// Package config загружает настройки harness из локального проекта и окружения.
package config

const StateDir = ".madharness-mini"

// ImageDetailValues перечисляет detail-режимы, которые можно отправить vision API.
var ImageDetailValues = map[string]bool{
	"auto":     true,
	"high":     true,
	"low":      true,
	"original": true,
}

// OrchestrationModeValues перечисляет режимы видимости delegate_task.
var OrchestrationModeValues = map[string]bool{
	"off":       true,
	"requested": true,
	"auto":      true,
	"required":  true,
}

// ApprovalModeValues перечисляет режимы обработки эскалируемых отказов policy.
var ApprovalModeValues = map[string]bool{
	"ask":  true,
	"deny": true,
}

// DefaultSettings задаёт минимальную рабочую конфигурацию текущей учебной ветки.
func DefaultSettings() Settings {
	return Settings{
		Model:                    "deepseek/deepseek-v4-flash",
		BaseURL:                  "https://openrouter.ai/api/v1",
		APIKey:                   "",
		Temperature:              0.2,
		MaxTurns:                 50,
		MaxParallelToolCalls:     1,
		MaxParallelSubagents:     1,
		ContextMaxTokens:         60000,
		ContextKeepRecentTurns:   3,
		WorkspaceRoot:            ".",
		ProtectedPaths:           []string{".git", ".env", "secrets", "~/.ssh"},
		AllowShell:               true,
		ApprovalMode:             "deny",
		YoloMode:                 false,
		OrchestrationEnabled:     true,
		OrchestrationMode:        "auto",
		SubagentMaxTurns:         10,
		SubagentContextMaxTokens: 30000,
		SupportsImageInput:       false,
		MaxImageBytes:            5_000_000,
		ImageDetail:              "auto",
		Headers:                  map[string]string{},
	}
}
