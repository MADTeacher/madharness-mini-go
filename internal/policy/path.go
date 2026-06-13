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

// New создаёт policy из эффективной конфигурации запуска.
func New(cfg *config.Config) *Policy {
	return &Policy{cfg: cfg, root: filepath.Clean(cfg.Root), protected: cfg.Data.ProtectedPaths}
}

// SafePath возвращает абсолютный путь внутри workspace или причину отказа.
func (p *Policy) SafePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.root, raw)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(p.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside workspace: %s", raw)
	}
	if p.isProtected(path, rel) {
		return "", fmt.Errorf("protected path: %s", raw)
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
