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

// DefaultSettings задаёт минимальную рабочую конфигурацию текущей учебной ветки.
func DefaultSettings() Settings {
	return Settings{
		Model:              "deepseek/deepseek-v4-flash",
		BaseURL:            "https://openrouter.ai/api/v1",
		APIKey:             "",
		Temperature:        0.2,
		MaxTurns:           50,
		WorkspaceRoot:      ".",
		ProtectedPaths:     []string{".git", ".env", "secrets", "~/.ssh"},
		AllowShell:         true,
		SupportsImageInput: false,
		MaxImageBytes:      5_000_000,
		ImageDetail:        "auto",
		Headers:            map[string]string{},
	}
}
