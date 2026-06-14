package subagents

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var (
	subagentNameRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)
	profileValues  = map[string]bool{"read-only": true, "writable": true}
)

func parseTools(raw any, location string, diagnostics *[]Diagnostic) []string {
	text := scalar(raw)
	if text == "" {
		*diagnostics = append(*diagnostics, Diagnostic{"error", location, "missing required tools"})
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(text), &values); err != nil {
		*diagnostics = append(*diagnostics, Diagnostic{"error", location, "tools must be a JSON-style list of strings: " + err.Error()})
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if seen[name] {
			*diagnostics = append(*diagnostics, Diagnostic{"error", location, "duplicate tool in tools"})
		}
		validateToolName(name, location, diagnostics)
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func optionalPositiveInt(raw any, name string, location string, diagnostics *[]Diagnostic) int {
	text := scalar(raw)
	if text == "" {
		return 0
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		*diagnostics = append(*diagnostics, Diagnostic{"error", location, name + " must be an integer"})
		return 0
	}
	if value <= 0 {
		*diagnostics = append(*diagnostics, Diagnostic{"error", location, name + " must be positive"})
		return 0
	}
	return value
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

func boolField(value any, fallback bool) bool {
	text := scalar(value)
	if text == "" {
		return fallback
	}
	switch strings.ToLower(text) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func validName(name string) bool {
	return subagentNameRE.MatchString(name) && !strings.Contains(name, "--")
}

func hasErrors(diagnostics []Diagnostic) bool {
	for _, item := range diagnostics {
		if item.Severity == "error" {
			return true
		}
	}
	return false
}

func emptyLabel(value string) string {
	if value == "" {
		return "<empty>"
	}
	return value
}
