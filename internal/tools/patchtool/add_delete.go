package patchtool

import (
	"fmt"
	"strings"
)

func (p *Parser) parseAdd(lines []string, i int) (int, error) {
	raw := strings.TrimPrefix(lines[i], "*** Add File: ")
	path, err := p.patchPath(raw)
	if err != nil {
		return i, err
	}
	if fileExists(path) || p.seen[path] {
		return i, fmt.Errorf("file already exists: %s", raw)
	}
	p.seen[path] = true
	i++
	newLines := []string{}
	for i < len(lines)-1 && !strings.HasPrefix(lines[i], "*** ") {
		if !strings.HasPrefix(lines[i], "+") {
			return i, fmt.Errorf("add file lines must start with +")
		}
		newLines = append(newLines, strings.TrimPrefix(lines[i], "+"))
		i++
	}
	content := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		content += "\n"
	}
	p.changes = append(p.changes, Change{Path: path, Content: contentPtr(content)})
	return i, nil
}

func (p *Parser) parseDelete(lines []string, i int) (int, error) {
	raw := strings.TrimPrefix(lines[i], "*** Delete File: ")
	path, err := p.patchPath(raw)
	if err != nil {
		return i, err
	}
	if err := p.mark(path, raw); err != nil {
		return i, err
	}
	if !fileExists(path) {
		return i, fmt.Errorf("not a file: %s", raw)
	}
	p.changes = append(p.changes, Change{Path: path})
	return i + 1, nil
}
