package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

const MaxOutput = 20000

// Clipped ограничивает большой tool output перед отправкой модели.
func Clipped(text string, limit int) string {
	if limit <= 0 {
		limit = MaxOutput
	}
	if len(text) <= limit {
		return text
	}
	return text[:limit] + fmt.Sprintf("\n...[clipped %d chars]", len(text)-limit)
}

// Ignored пропускает служебные каталоги при поиске и листинге.
func Ignored(path string) bool {
	ignored := map[string]bool{
		".git":             true,
		".madharness-mini": true,
		"__pycache__":      true,
		".venv":            true,
		".uv-cache":        true,
	}
	for _, part := range splitPath(path) {
		if ignored[part] {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return nil
	}
	parts := []string{}
	for _, part := range strings.Split(cleaned, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
