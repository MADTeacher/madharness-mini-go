// Package subagents загружает markdown-роли и описывает делегацию parent agent.
package subagents

import (
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostic описывает ошибку или предупреждение при discovery субагентов.
type Diagnostic struct {
	Severity string
	Path     string
	Message  string
}

// AsMap готовит диагностику для CLI и trace.
func (d Diagnostic) AsMap() map[string]string {
	return map[string]string{
		"severity": d.Severity,
		"path":     filepath.ToSlash(d.Path),
		"message":  d.Message,
	}
}

// Subagent хранит валидную markdown-роль и её системный prompt.
type Subagent struct {
	Name             string
	Description      string
	Profile          string
	Tools            []string
	Prompt           string
	Source           string
	Location         string
	MaxTurns         int
	ContextMaxTokens int
	Metadata         map[string]string
	Builtin          bool
	Override         bool
}

// Index содержит найденные роли и диагностику загрузки.
type Index struct {
	Subagents   map[string]Subagent
	Diagnostics []Diagnostic
}

// Names возвращает имена ролей в стабильном порядке.
func (i Index) Names() []string {
	names := make([]string, 0, len(i.Subagents))
	for name := range i.Subagents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func displayPath(path string, workspaceRoot string) string {
	if path == "" {
		return ""
	}
	rel, err := filepath.Rel(workspaceRoot, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}
