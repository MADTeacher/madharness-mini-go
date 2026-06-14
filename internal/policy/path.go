// Package policy проверяет границы workspace перед файловыми и shell-действиями.
package policy

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

// Policy решает, какие пути и команды доступны агенту в текущем workspace.
type Policy struct {
	cfg       *config.Config
	root      string
	protected []string
}

// PathDecision добавляет к решению нормализованный путь внутри workspace.
type PathDecision struct {
	Decision
	Path string
}

// New создаёт policy из эффективной конфигурации запуска.
func New(cfg *config.Config) *Policy {
	return &Policy{cfg: cfg, root: filepath.Clean(cfg.Root), protected: cfg.Data.ProtectedPaths}
}

// SafePath возвращает абсолютный путь внутри workspace или причину отказа.
func (p *Policy) SafePath(raw string) (string, error) {
	decision := p.SafePathDecision(raw)
	if !decision.Allowed {
		return "", fmt.Errorf("%s", decision.Reason)
	}
	return decision.Path, nil
}

// SafePathDecision проверяет путь и помечает protected path как эскалируемый.
func (p *Policy) SafePathDecision(raw string) PathDecision {
	if raw == "" {
		return PathDecision{Decision: deny("empty_path", "empty path", false)}
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.root, raw)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(p.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return PathDecision{Decision: deny(CodePathOutsideWorkspace, "path outside workspace: "+raw, false)}
	}
	if p.isProtected(path, rel) {
		return PathDecision{Decision: deny(CodeProtectedPath, "protected path: "+raw, true), Path: path}
	}
	return PathDecision{Decision: allow(), Path: path}
}

// SkillRoot проверяет фиксированный каталог skills внутри workspace.
//
// Discovery читает только заранее известные skill roots. Они не проходят через
// protected_paths как пользовательские файловые инструменты, но всё равно не
// могут выходить за workspace.
func (p *Policy) SkillRoot(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty skill root")
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.root, raw)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(p.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("skill root outside workspace: %s", raw)
	}
	return path, nil
}

func (p *Policy) isProtected(path string, rel string) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	for _, item := range p.protected {
		if item == "" {
			continue
		}
		expanded := expandHome(item)
		if filepath.IsAbs(expanded) {
			if pathWithin(path, filepath.Clean(expanded)) {
				return true
			}
			continue
		}
		name := filepath.Base(strings.Trim(item, "/"))
		for _, part := range parts {
			if name != "" && part == name {
				return true
			}
		}
	}
	return false
}

func pathWithin(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
