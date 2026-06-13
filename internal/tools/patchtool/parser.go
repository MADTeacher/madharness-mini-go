package patchtool

import (
	"fmt"
	"os"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// Change описывает итог одного файлового изменения после разбора patch.
type Change struct {
	Path    string
	Content *string
}

// Parser валидирует patch целиком до записи на диск.
type Parser struct {
	ctx     *tools.Context
	seen    map[string]bool
	changes []Change
}

// Prepare разбирает patch и возвращает упорядоченный набор изменений.
func (p *Parser) Prepare(patch string) ([]Change, error) {
	lines := splitPatchLines(patch)
	if len(lines) == 0 || lines[0] != "*** Begin Patch" {
		return nil, fmt.Errorf("patch must start with *** Begin Patch")
	}
	if lines[len(lines)-1] != "*** End Patch" {
		return nil, fmt.Errorf("patch must end with *** End Patch")
	}
	p.seen = map[string]bool{}
	for i := 1; i < len(lines)-1; {
		var err error
		switch line := lines[i]; {
		case strings.HasPrefix(line, "*** Add File: "):
			i, err = p.parseAdd(lines, i)
		case strings.HasPrefix(line, "*** Update File: "):
			i, err = p.parseUpdate(lines, i)
		case strings.HasPrefix(line, "*** Delete File: "):
			i, err = p.parseDelete(lines, i)
		case strings.HasPrefix(line, "*** Move to: "):
			err = fmt.Errorf("Move to is only supported after Update File")
		case line == "*** End of File":
			i++
		default:
			err = fmt.Errorf("unexpected patch line: %s", line)
		}
		if err != nil {
			return nil, err
		}
	}
	return p.changes, nil
}

func splitPatchLines(patch string) []string {
	trimmed := strings.TrimRight(patch, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
}

func (p *Parser) patchPath(raw string) (string, error) {
	if scopeError := p.ctx.WritePathError(raw); scopeError != "" {
		return "", fmt.Errorf("%s", scopeError)
	}
	return p.ctx.Policy.SafePath(raw)
}

func (p *Parser) mark(path string, raw string) error {
	if p.seen[path] {
		return fmt.Errorf("file changed more than once: %s", raw)
	}
	p.seen[path] = true
	return nil
}

func contentPtr(content string) *string {
	value := content
	return &value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
