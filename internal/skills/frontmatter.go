package skills

import (
	"fmt"
	"regexp"
	"strings"
)

var skillNameRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

func parseSkillMarkdown(text string) (map[string]any, string, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return nil, "", fmt.Errorf("SKILL.md frontmatter is not closed")
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
		if isBlockScalar(value) {
			block, next := readBlockScalar(lines, index+1, strings.HasPrefix(value, ">"))
			fields[key] = block
			index = next
			continue
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

func validSkillName(name string) bool {
	return skillNameRE.MatchString(name) && !strings.Contains(name, "--")
}

func scalar(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func metadata(value any) map[string]string {
	raw, ok := value.(map[string]string)
	if !ok {
		return map[string]string{}
	}
	out := map[string]string{}
	for key, item := range raw {
		out[key] = item
	}
	return out
}

func isBlockScalar(value string) bool {
	switch value {
	case "|", "|-", "|+", ">", ">-", ">+":
		return true
	default:
		return false
	}
}

func readBlockScalar(lines []string, index int, folded bool) (string, int) {
	collected := []string{}
	for index < len(lines) {
		raw := lines[index]
		if raw != "" && raw[0] != ' ' && raw[0] != '\t' {
			break
		}
		if strings.HasPrefix(raw, "  ") {
			collected = append(collected, raw[2:])
		} else {
			collected = append(collected, strings.TrimLeft(raw, " \t"))
		}
		index++
	}
	if folded {
		parts := []string{}
		for _, line := range collected {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
		return strings.Join(parts, " "), index
	}
	return strings.TrimSpace(strings.Join(collected, "\n")), index
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
