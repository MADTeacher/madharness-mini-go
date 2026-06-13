// Package instructions загружает проектные правила, которые дополняют system prompt.
package instructions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

const (
	// ProjectDocFilename — устойчивое имя файла правил в корне workspace.
	ProjectDocFilename = "AGENTS.md"
	// ProjectDocMaxBytes ограничивает вклад локальных правил в system prompt.
	ProjectDocMaxBytes = 32 * 1024
)

// LoadProject читает корневой AGENTS.md текущего workspace.
func LoadProject(cfg *config.Config) (string, error) {
	path := filepath.Join(cfg.Root, ProjectDocFilename)
	raw, err := os.ReadFile(path)
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
