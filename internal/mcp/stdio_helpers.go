package mcp

import (
	"fmt"
	"os"
	"strings"
)

var safeInheritedEnv = map[string]bool{
	"ComSpec":    true,
	"HOME":       true,
	"LANG":       true,
	"LC_ALL":     true,
	"PATH":       true,
	"SystemRoot": true,
	"TEMP":       true,
	"TMP":        true,
	"TMPDIR":     true,
	"USER":       true,
	"WINDIR":     true,
}

func toolsFromResult(result map[string]any) ([]map[string]any, error) {
	rawTools, ok := result["tools"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid MCP tools/list response: tools must be list")
	}
	out := make([]map[string]any, 0, len(rawTools))
	for _, raw := range rawTools {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid MCP tools/list response: tool must be object")
		}
		out = append(out, item)
	}
	return out, nil
}

func serverEnv(explicit map[string]string) []string {
	env := []string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || strings.HasPrefix(key, "MADHARNESS_MINI_") || !safeInheritedEnv[key] {
			continue
		}
		env = append(env, key+"="+value)
	}
	for key, value := range explicit {
		env = append(env, key+"="+value)
	}
	return env
}

func isServerRequest(message map[string]any) bool {
	_, hasID := message["id"]
	_, hasMethod := message["method"]
	_, hasResult := message["result"]
	_, hasError := message["error"]
	return hasID && hasMethod && !hasResult && !hasError
}

func clipText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + fmt.Sprintf("\n...[clipped %d chars]", len(text)-limit)
}
