package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	var contextReport map[string]any
	var discovered map[string]any
	activated := map[string]bool{}
	resourcesUsed := 0
	mcpStarted := map[string]int{}
	mcpStopped := 0
	mcpErrors := 0
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
		if event["event"] == "skills_discovered" {
			discovered = event
		}
		if event["event"] == "skill_activated" {
			activated[fmt.Sprint(event["name"])] = true
		}
		if event["event"] == "skill_resource_used" {
			resourcesUsed++
		}
		if event["event"] == "mcp_server_started" {
			mcpStarted[fmt.Sprint(event["server"])] = intFromAny(event["tools_count"])
		}
		if event["event"] == "mcp_server_stopped" {
			mcpStopped++
		}
		if event["event"] == "mcp_server_error" {
			mcpErrors++
		}
		if report, ok := event["context_report"].(map[string]any); ok {
			contextReport = report
		}
	}
	result = truncateRunes(result, 1000)
	lines := []string{
		fmt.Sprintf("trace: %s", path),
		fmt.Sprintf("events: %d", len(events)),
		fmt.Sprintf("tool calls: %d", toolCalls),
	}
	if contextReport != nil {
		lines = append(lines, summarizeContextReport(contextReport))
	}
	if discovered != nil || len(activated) > 0 || resourcesUsed > 0 {
		names := make([]string, 0, len(activated))
		for name := range activated {
			if name != "" && name != "<nil>" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		activatedText := "none"
		if len(names) > 0 {
			activatedText = strings.Join(names, ", ")
		}
		lines = append(lines, fmt.Sprintf(
			"skills: discovered %d; activated %s; resources used %d",
			intFromAny(discovered["count"]),
			activatedText,
			resourcesUsed,
		))
	}
	if len(mcpStarted) > 0 || mcpStopped > 0 || mcpErrors > 0 {
		servers := make([]string, 0, len(mcpStarted))
		for server, count := range mcpStarted {
			servers = append(servers, fmt.Sprintf("%s(%d tools)", server, count))
		}
		sort.Strings(servers)
		startedText := "none"
		if len(servers) > 0 {
			startedText = strings.Join(servers, ", ")
		}
		lines = append(lines, fmt.Sprintf(
			"mcp: started %s; stopped %d; errors %d",
			startedText,
			mcpStopped,
			mcpErrors,
		))
	}
	lines = append(lines, fmt.Sprintf("result: %s", result))
	return strings.Join(lines, "\n"), nil
}

func summarizeContextReport(report map[string]any) string {
	history, _ := report["history"].(map[string]any)
	requestTokens := intFromAny(report["request_tokens_estimate"])
	maxTokens := intFromAny(report["max_tokens"])
	toolsTokens := intFromAny(report["tools_tokens_estimate"])
	fragments, _ := report["fragments"].([]any)
	totalEntries := intFromAny(history["total_entries"])
	renderedEntries := intFromAny(history["rendered_entries"])
	clipped := lenFromAny(history["clipped_tool_messages"])
	dropped := lenFromAny(history["dropped_entries"])
	return fmt.Sprintf(
		"context: %d/%d estimated tokens; tools: %d; fragments: %d; history: %d/%d entries; clipped tool messages: %d; dropped entries: %d",
		requestTokens,
		maxTokens,
		toolsTokens,
		len(fragments),
		renderedEntries,
		totalEntries,
		clipped,
		dropped,
	)
}

func intFromAny(value any) int {
	switch item := value.(type) {
	case int:
		return item
	case int64:
		return int(item)
	case float64:
		return int(item)
	default:
		return 0
	}
}

func lenFromAny(value any) int {
	switch items := value.(type) {
	case []any:
		return len(items)
	case []map[string]any:
		return len(items)
	default:
		return 0
	}
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
