// Package config загружает настройки harness из локального проекта и окружения.
package config

const StateDir = ".madharness-mini"

// DefaultSettings задаёт минимальную рабочую конфигурацию первой учебной ветки.
func DefaultSettings() Settings {
	return Settings{
		Model:          "deepseek/deepseek-v4-flash",
		BaseURL:        "https://openrouter.ai/api/v1",
		APIKey:         "",
		Temperature:    0.2,
		MaxTurns:       50,
		WorkspaceRoot:  ".",
		ProtectedPaths: []string{".git", ".env", "secrets", "~/.ssh"},
		AllowShell:     true,
		Headers:        map[string]string{},
	}
}
