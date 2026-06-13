package patchtool

import (
	"fmt"
	"os"
	"strings"
)

func (p *Parser) parseUpdate(lines []string, i int) (int, error) {
	raw := strings.TrimPrefix(lines[i], "*** Update File: ")
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
	i++
	targetPath, err := p.parseMoveTarget(lines, &i)
	if err != nil {
		return i, err
	}
	originalBytes, err := os.ReadFile(path)
	if err != nil {
		return i, err
	}
	original := string(originalBytes)
	current := splitOriginalLines(original)
	sawHunk := false
	for i < len(lines)-1 && !strings.HasPrefix(lines[i], "*** ") {
		if strings.HasPrefix(lines[i], "@@") {
			i++
		}
		var oldLines, newLines []string
		for i < len(lines)-1 && !strings.HasPrefix(lines[i], "@@") && !strings.HasPrefix(lines[i], "*** ") {
			if lines[i] == "" {
				return i, fmt.Errorf("invalid hunk line: ")
			}
			marker, content := lines[i][:1], lines[i][1:]
			switch marker {
			case " ":
				oldLines = append(oldLines, content)
				newLines = append(newLines, content)
			case "-":
				oldLines = append(oldLines, content)
			case "+":
				newLines = append(newLines, content)
			default:
				return i, fmt.Errorf("invalid hunk line: %s", lines[i])
			}
			i++
		}
		if len(oldLines) == 0 && len(newLines) == 0 {
			return i, fmt.Errorf("empty update hunk")
		}
		current, err = applyHunk(current, oldLines, newLines)
		if err != nil {
			return i, err
		}
		sawHunk = true
	}
	return i, p.recordUpdate(path, raw, targetPath, original, current, sawHunk)
}

func splitOriginalLines(original string) []string {
	if original == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(original, "\r\n", "\n"), "\n")
	if strings.HasSuffix(original, "\n") && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (p *Parser) parseMoveTarget(lines []string, i *int) (string, error) {
	if *i >= len(lines)-1 || !strings.HasPrefix(lines[*i], "*** Move to: ") {
		return "", nil
	}
	targetRaw := strings.TrimPrefix(lines[*i], "*** Move to: ")
	targetPath, err := p.patchPath(targetRaw)
	if err != nil {
		return "", err
	}
	if fileExists(targetPath) || p.seen[targetPath] {
		return "", fmt.Errorf("target file already exists: %s", targetRaw)
	}
	p.seen[targetPath] = true
	*i = *i + 1
	return targetPath, nil
}

func (p *Parser) recordUpdate(path, raw, targetPath, original string, current []string, sawHunk bool) error {
	if !sawHunk && targetPath == "" {
		return fmt.Errorf("update has no hunks: %s", raw)
	}
	updated := original
	if sawHunk {
		updated = strings.Join(current, "\n")
		if strings.HasSuffix(original, "\n") {
			updated += "\n"
		}
	}
	if targetPath == "" {
		p.changes = append(p.changes, Change{Path: path, Content: contentPtr(updated)})
		return nil
	}
	p.changes = append(p.changes, Change{Path: path}, Change{Path: targetPath, Content: contentPtr(updated)})
	return nil
}
