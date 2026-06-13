package subagents

import (
	"fmt"
	"strings"
)

func parseMarkdown(text string) (map[string]any, string, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("subagent markdown must start with YAML frontmatter")
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return nil, "", fmt.Errorf("subagent frontmatter is not closed")
	}
	fields, err := parseFrontmatterLines(lines[1:closing])
	if err != nil {
		return nil, "", err
	}
	return fields, strings.Join(lines[closing+1:], "\n"), nil
}

func parseFrontmatterLines(lines []string) (map[string]any, error) {
	fields := map[string]any{}
	for index := 0; index < len(lines); {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(strings.TrimLeft(raw, " \t"), "#") {
			index++
			continue
		}
		if raw[0] == ' ' || raw[0] == '\t' {
			return nil, fmt.Errorf("unexpected indented frontmatter line: %s", trimmed)
		}
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			return nil, fmt.Errorf("invalid frontmatter line: %s", trimmed)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("empty frontmatter key")
		}
		if key == "metadata" && value == "" {
			mapping, next, err := readIndentedMapping(lines, index+1)
			if err != nil {
				return nil, err
			}
			fields[key] = mapping
			index = next
			continue
		}
		fields[key] = unquote(value)
		index++
	}
	return fields, nil
}

func readIndentedMapping(lines []string, index int) (map[string]string, int, error) {
	mapping := map[string]string{}
	for index < len(lines) {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			index++
			continue
		}
		if raw[0] != ' ' && raw[0] != '\t' {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, index, fmt.Errorf("invalid metadata line: %s", trimmed)
		}
		mapping[strings.TrimSpace(key)] = unquote(strings.TrimSpace(value))
		index++
	}
	return mapping, index, nil
}

func unquote(value string) string {
	if len(value) >= 2 {
		first := value[0]
		if first == value[len(value)-1] && (first == '\'' || first == '"') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func scalar(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}
