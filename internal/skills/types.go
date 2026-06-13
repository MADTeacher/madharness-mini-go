// Package skills ищет project-local Agent Skills и готовит их для run-режима.
package skills

import (
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostic описывает ошибку или предупреждение при поиске SKILL.md.
type Diagnostic struct {
	Severity string
	Path     string
	Message  string
}

// AsMap готовит диагностику для CLI и trace без лишних абсолютных путей.
func (d Diagnostic) AsMap(workspaceRoot string) map[string]string {
	return map[string]string{
		"severity": d.Severity,
		"path":     displayPath(d.Path, workspaceRoot),
		"message":  d.Message,
	}
}

// Resource описывает один bundled-файл внутри корня skill.
type Resource struct {
	RelativePath  string
	WorkspacePath string
	Kind          string
	Bytes         int64
}

// AsMap отдаёт JSON-friendly описание ресурса без чтения содержимого.
func (r Resource) AsMap() map[string]any {
	return map[string]any{
		"path":           r.RelativePath,
		"workspace_path": r.WorkspacePath,
		"kind":           r.Kind,
		"bytes":          r.Bytes,
	}
}

// Skill хранит валидный SKILL.md и безопасные пути к его каталогу.
type Skill struct {
	Name          string
	Description   string
	Root          string
	SkillFile     string
	Body          string
	RawText       string
	Source        string
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  []string
	Warnings      []string
}

// Location показывает путь к SKILL.md относительно workspace, если это возможно.
func (s Skill) Location(workspaceRoot string) string {
	return displayPath(s.SkillFile, workspaceRoot)
}

// RootLocation показывает каталог skill относительно workspace.
func (s Skill) RootLocation(workspaceRoot string) string {
	return displayPath(s.Root, workspaceRoot)
}

// Index содержит найденные skills и диагностику сканирования.
type Index struct {
	Skills      map[string]Skill
	Diagnostics []Diagnostic
}

// Names возвращает имена skills в стабильном порядке для CLI и JSON Schema enum.
func (i Index) Names() []string {
	names := make([]string, 0, len(i.Skills))
	for name := range i.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// NameSet возвращает быстрый набор доступных имён.
func (i Index) NameSet() map[string]bool {
	out := map[string]bool{}
	for name := range i.Skills {
		out[name] = true
	}
	return out
}

func displayPath(path string, workspaceRoot string) string {
	if path == "" {
		return ""
	}
	rel, err := filepath.Rel(workspaceRoot, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	if resolvedRoot, err := filepath.EvalSymlinks(workspaceRoot); err == nil {
		if rel, err := filepath.Rel(resolvedRoot, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}
