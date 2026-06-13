package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

// Summarize строит короткую CLI-сводку по trace id или его префиксу.
func Summarize(cfg *config.Config, traceID string) (string, error) {
	pattern := filepath.Join(cfg.StateDir, "traces", traceID+"*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("trace not found: %s", traceID)
	}
	path := matches[0]
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	events, err := parseEvents(raw)
	if err != nil {
		return "", err
	}
	toolCalls := 0
	result := ""
	for i := len(events) - 1; i >= 0; i-- {
		if events[i]["event"] == "session_end" {
			result = fmt.Sprint(events[i]["result"])
			break
		}
	}
	for _, event := range events {
		if event["event"] == "tool_observation" {
			toolCalls++
		}
	}
	result = truncateRunes(result, 1000)
	return fmt.Sprintf(
		"trace: %s\nevents: %d\ntool calls: %d\nresult: %s",
		path,
		len(events),
		toolCalls,
		result,
	), nil
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func parseEvents(raw []byte) ([]map[string]any, error) {
	events := []map[string]any{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
