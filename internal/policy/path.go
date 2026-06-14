// Package policy проверяет границы workspace перед файловыми и shell-действиями.
package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

// Policy решает, какие пути и команды доступны агенту в текущем workspace.
type Policy struct {
	cfg          *config.Config
	root         string
	resolvedRoot string
	protected    []string
}

// PathDecision добавляет к решению нормализованный путь внутри workspace.
type PathDecision struct {
	Decision
	Path string
}

// New создаёт policy из эффективной конфигурации запуска.
func New(cfg *config.Config) *Policy {
	root := filepath.Clean(cfg.Root)
	resolvedRoot := root
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		resolvedRoot = filepath.Clean(realRoot)
	}
	return &Policy{cfg: cfg, root: root, resolvedRoot: resolvedRoot, protected: cfg.Data.ProtectedPaths}
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
	resolved, err := resolvePathForDecision(path)
	if err != nil {
		return PathDecision{Decision: deny(CodePathOutsideWorkspace, "path cannot be resolved safely: "+raw+": "+err.Error(), false)}
	}
	resolvedRel, err := filepath.Rel(p.resolvedRoot, resolved)
	if err != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
		return PathDecision{Decision: deny(CodePathOutsideWorkspace, "path outside workspace: "+raw, false)}
	}
	if p.isProtected(path, rel) || p.isProtected(resolved, resolvedRel) {
		return PathDecision{Decision: deny(CodeProtectedPath, "protected path: "+raw, true), Path: path}
	}
	return PathDecision{Decision: allow(), Path: path}
}

// TrustedWorkspacePath проверяет заранее известный control-plane путь harness.
//
// Discovery читает только фиксированные roots. Они не проходят через
// protected_paths как пользовательские файловые инструменты, но всё равно не
// могут выходить за workspace, в том числе через symlink.
func (p *Policy) TrustedWorkspacePath(raw string, label string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty %s", label)
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.root, raw)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(p.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s outside workspace: %s", label, raw)
	}
	resolved, err := resolvePathForDecision(path)
	if err != nil {
		return "", fmt.Errorf("%s cannot be resolved safely: %s: %w", label, raw, err)
	}
	resolvedRel, err := filepath.Rel(p.resolvedRoot, resolved)
	if err != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s outside workspace: %s", label, raw)
	}
	return path, nil
}

// SkillRoot проверяет фиксированный каталог skills внутри workspace.
func (p *Policy) SkillRoot(raw string) (string, error) {
	return p.TrustedWorkspacePath(raw, "skill root")
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
		cleaned := filepath.Clean(expanded)
		if cleaned == "." {
			continue
		}
		if strings.Contains(cleaned, string(filepath.Separator)) {
			if rel == cleaned || strings.HasPrefix(rel, cleaned+string(filepath.Separator)) {
				return true
			}
			continue
		}
		name := filepath.Base(strings.Trim(cleaned, "/"))
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

func resolvePathForDecision(path string) (string, error) {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	current := path
	missing := []string{}
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
