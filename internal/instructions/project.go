// Package instructions загружает проектные правила, которые дополняют system prompt.
package instructions

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

const (
	// ProjectDocFilename — устойчивое имя файла правил в корне workspace.
	ProjectDocFilename = "AGENTS.md"
	// ProjectDocMaxBytes ограничивает вклад локальных правил в system prompt.
	ProjectDocMaxBytes = 32 * 1024
)

// LoadProject читает корневой AGENTS.md текущего workspace.
func LoadProject(cfg *config.Config) (string, error) {
	path, err := safeProjectDocPath(cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	raw, err := readProjectDocPrefix(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	text := strings.TrimRightFunc(string(raw), unicode.IsSpace)
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	data := []byte(text)
	if len(data) > ProjectDocMaxBytes {
		data = data[:ProjectDocMaxBytes]
		for len(data) > 0 && !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
	}
	return strings.TrimRightFunc(string(data), unicode.IsSpace), nil
}

func safeProjectDocPath(cfg *config.Config) (string, error) {
	path := filepath.Join(cfg.Root, ProjectDocFilename)
	resolvedRoot := filepath.Clean(cfg.Root)
	if realRoot, err := filepath.EvalSymlinks(cfg.Root); err == nil {
		resolvedRoot = filepath.Clean(realRoot)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside workspace: %s", ProjectDocFilename)
	}
	if filepath.ToSlash(rel) == ProjectDocFilename {
		return resolved, nil
	}
	decision := policy.New(cfg).SafePathDecision(filepath.ToSlash(rel))
	if !decision.Allowed {
		return "", fmt.Errorf("%s", decision.Reason)
	}
	return resolved, nil
}

func readProjectDocPrefix(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project instructions is not a regular file: %s", filepath.Base(path))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, ProjectDocMaxBytes+1))
}
