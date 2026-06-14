package subagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var patchPathRE = regexp.MustCompile(`^\*\*\* (?:Add|Update|Delete) File: (.+)$`)
var patchMoveRE = regexp.MustCompile(`^\*\*\* Move to: (.+)$`)

// TracePathForObservation показывает trace относительно cwd, если это возможно.
func TracePathForObservation(path string, cwd string) string {
	if rel, err := filepath.Rel(cwd, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// Summary описывает дочернюю трассу без полного JSONL.
type Summary struct {
	ToolCalls    int
	ModelCalls   int
	ChangedFiles []string
	Text         string
}

// SummarizeTrace собирает короткую сводку локальной трассы субагента.
func SummarizeTrace(path string) Summary {
	events := readTraceEvents(path)
	toolCalls := 0
	modelCalls := 0
	for _, event := range events {
		switch event["event"] {
		case "tool_observation":
			toolCalls++
		case "model_call_finished":
			modelCalls++
		}
	}
	changed := ChangedFilesFromEvents(events)
	return Summary{
		ToolCalls:    toolCalls,
		ModelCalls:   modelCalls,
		ChangedFiles: changed,
		Text: fmt.Sprintf(
			"%d model calls, %d tool calls, %d changed files",
			modelCalls,
			toolCalls,
			len(changed),
		),
	}
}

// ChangedFilesFromEvents достаёт пути из write_file и apply_patch.
func ChangedFilesFromEvents(events []map[string]any) []string {
	seen := map[string]bool{}
	for _, event := range events {
		if event["event"] != "tool_observation" {
			continue
		}
		observation, _ := event["observation"].(map[string]any)
		if observation["ok"] != true {
			continue
		}
		toolName, _ := event["tool"].(string)
		args, _ := event["args"].(map[string]any)
		switch toolName {
		case "write_file":
			if path, ok := args["path"].(string); ok && path != "" {
				seen[path] = true
			}
		case "apply_patch":
			if patch, ok := args["patch"].(string); ok {
				for _, path := range pathsFromPatch(patch) {
					seen[path] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func readTraceEvents(path string) []map[string]any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	events := []map[string]any{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			events = append(events, event)
		}
	}
	return events
}

func pathsFromPatch(patch string) []string {
	paths := []string{}
	for _, line := range strings.Split(patch, "\n") {
		if match := patchPathRE.FindStringSubmatch(line); len(match) == 2 {
			paths = append(paths, strings.TrimSpace(match[1]))
			continue
		}
		if match := patchMoveRE.FindStringSubmatch(line); len(match) == 2 {
			paths = append(paths, strings.TrimSpace(match[1]))
		}
	}
	return paths
}
