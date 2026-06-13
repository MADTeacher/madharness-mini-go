// Package prompt отдаёт встроенные системные инструкции модели.
package prompt

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed prompts/*.md
var prompts embed.FS

// Load читает markdown prompt по имени без завершающих пустых строк.
func Load(name string) (string, error) {
	data, err := prompts.ReadFile("prompts/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("load prompt %q: %w", name, err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}
