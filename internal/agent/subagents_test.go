package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDelegateTaskRunsWritableSubagentWithLocalTrace(t *testing.T) {
	cfg := testAgentConfig(t)
	writeAgentSubagent(t, cfg.Root, "test-writer", "writable", []string{"list_files", "read_file", "search_code", "write_file"})
	client := &sequenceClient{responses: []map[string]any{
		toolCallResponse("call_delegate", "delegate_task", map[string]any{"subagent": "test-writer", "task": "write result"}),
		toolCallResponse("call_write", "write_file", map[string]any{"path": "subagent-result.txt", "content": "done\n"}),
		contentResponse("subagent done"),
		contentResponse("parent done"),
	}}

	result, tracePath, err := runWithClientOptions("delegate work", cfg, client, RunOptions{OrchestrationMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}

	if result != "parent done" {
		t.Fatalf("result = %q", result)
	}
	if text := readFileText(t, filepath.Join(cfg.Root, "subagent-result.txt")); text != "done\n" {
		t.Fatalf("subagent-result.txt = %q", text)
	}
	if !containsTool(client.toolsSeen[0], "delegate_task") || containsTool(client.toolsSeen[1], "delegate_task") {
		t.Fatalf("toolsSeen = %+v", client.toolsSeen)
	}
	events := readTraceEvents(t, tracePath)
	finished := firstEvent(events, "subagent_finished")
	if finished == nil {
		t.Fatal("subagent_finished event not found")
	}
	childPath := filepath.Join(cfg.CWD, finished["trace_path"].(string))
	if _, err := os.Stat(childPath); err != nil {
		t.Fatalf("child trace missing: %v", err)
	}
	observation := delegateObservation(t, events)
	if observation["subagent"] != "test-writer" || observation["subagent_trace_id"] != finished["trace_id"] {
		t.Fatalf("delegate observation = %+v finished=%+v", observation, finished)
	}
	changed, _ := observation["changed_files"].([]any)
	if len(changed) != 1 || changed[0] != "subagent-result.txt" {
		t.Fatalf("changed_files = %+v", observation["changed_files"])
	}
}

func TestOrchestrationModesControlDelegateTask(t *testing.T) {
	cfg := testAgentConfig(t)
	client := &sequenceClient{responses: []map[string]any{
		contentResponse("off"),
		contentResponse("plain"),
		contentResponse("requested"),
		contentResponse("required"),
	}}

	if _, _, err := runWithClientOptions("обычная задача", cfg, client, RunOptions{OrchestrationMode: "off"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runWithClientOptions("обычная задача", cfg, client, RunOptions{OrchestrationMode: "requested"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runWithClientOptions("используй субагентов для проверки", cfg, client, RunOptions{OrchestrationMode: "requested"}); err != nil {
		t.Fatal(err)
	}
	if _, tracePath, err := runWithClientOptions("длинная задача", cfg, client, RunOptions{OrchestrationMode: "required"}); err != nil {
		t.Fatal(err)
	} else if mode := firstEvent(readTraceEvents(t, tracePath), "orchestration_mode"); mode["effective"] != "required" {
		t.Fatalf("mode = %+v", mode)
	}

	if containsTool(client.toolsSeen[0], "delegate_task") {
		t.Fatal("off mode exposed delegate_task")
	}
	if containsTool(client.toolsSeen[1], "delegate_task") {
		t.Fatal("requested mode without marker exposed delegate_task")
	}
	if !containsTool(client.toolsSeen[2], "delegate_task") {
		t.Fatal("requested mode with marker did not expose delegate_task")
	}
	requiredNames := schemaToolNames(client.toolsSeen[3])
	if strings.Join(requiredNames, ",") != "list_files,read_file,search_code,delegate_task" {
		t.Fatalf("required tools = %v", requiredNames)
	}
}

func TestPlannerCanRequestUserInput(t *testing.T) {
	cfg := testAgentConfig(t)
	client := &sequenceClient{responses: []map[string]any{
		toolCallResponse("call_delegate", "delegate_task", map[string]any{"subagent": "planner", "task": "choose path"}),
		toolCallResponse("call_question", "ask_user", map[string]any{
			"question": "Какой путь выбрать?",
			"options":  []string{"A", "B"},
			"reason":   "Нужно решение.",
		}),
	}}

	result, tracePath, err := runWithClientOptions("plan with question", cfg, client, RunOptions{OrchestrationMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"planner просит уточнение", "Какой путь выбрать?", "1. A", "2. B", "Причина: Нужно решение."} {
		if !strings.Contains(result, want) {
			t.Fatalf("result missing %q: %s", want, result)
		}
	}
	events := readTraceEvents(t, tracePath)
	observation := delegateObservation(t, events)
	if observation["status"] != "needs_user_input" || observation["question"] != "Какой путь выбрать?" {
		t.Fatalf("delegate observation = %+v", observation)
	}
	if firstEvent(events, "user_input_requested") == nil {
		t.Fatal("user_input_requested event not written")
	}
}

func TestPlannerCannotWriteNonMarkdownFiles(t *testing.T) {
	cfg := testAgentConfig(t)
	client := &sequenceClient{responses: []map[string]any{
		toolCallResponse("call_delegate", "delegate_task", map[string]any{"subagent": "planner", "task": "create index"}),
		toolCallResponse("call_write", "write_file", map[string]any{"path": "index.html", "content": "<html></html>\n"}),
		contentResponse("planner stopped"),
		contentResponse("parent done"),
	}}

	result, tracePath, err := runWithClientOptions("planner writes index", cfg, client, RunOptions{OrchestrationMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "parent done" {
		t.Fatalf("result = %q", result)
	}
	if _, err := os.Stat(filepath.Join(cfg.Root, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("index.html should not exist, err=%v", err)
	}
	observation := delegateObservation(t, readTraceEvents(t, tracePath))
	childPath := filepath.Join(cfg.CWD, observation["subagent_trace_path"].(string))
	childEvents := readTraceEvents(t, childPath)
	writeObservation := toolObservation(childEvents, "write_file")
	if writeObservation["ok"] != false || !strings.Contains(writeObservation["summary"].(string), "planner may write only Markdown plan files") {
		t.Fatalf("write observation = %+v", writeObservation)
	}
}

func TestDelegateProfileDowngradeAndUpgrade(t *testing.T) {
	cfg := testAgentConfig(t)
	writeAgentSubagent(t, cfg.Root, "test-writer", "writable", []string{"list_files", "write_file", "run_shell", "start_shell", "shell_status", "stop_shell"})
	downgradeClient := &sequenceClient{responses: []map[string]any{
		toolCallResponse("call_delegate", "delegate_task", map[string]any{"subagent": "test-writer", "task": "inspect", "profile": "read-only"}),
		contentResponse("subagent done"),
		contentResponse("parent done"),
	}}
	if _, _, err := runWithClientOptions("downgrade", cfg, downgradeClient, RunOptions{OrchestrationMode: "auto"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"write_file", "run_shell", "start_shell", "shell_status", "stop_shell"} {
		if containsTool(downgradeClient.toolsSeen[1], name) {
			t.Fatalf("downgraded tools = %v", schemaToolNames(downgradeClient.toolsSeen[1]))
		}
	}
	if strings.Join(schemaToolNames(downgradeClient.toolsSeen[1]), ",") != "list_files" {
		t.Fatalf("downgraded tools = %v", schemaToolNames(downgradeClient.toolsSeen[1]))
	}

	upgradeClient := &sequenceClient{responses: []map[string]any{
		toolCallResponse("call_delegate", "delegate_task", map[string]any{"subagent": "reviewer", "task": "review", "profile": "writable"}),
		contentResponse("parent done"),
	}}
	_, tracePath, err := runWithClientOptions("upgrade", cfg, upgradeClient, RunOptions{OrchestrationMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	observation := delegateObservation(t, readTraceEvents(t, tracePath))
	if observation["ok"] != false || !strings.Contains(observation["summary"].(string), "subagent is not writable") {
		t.Fatalf("upgrade observation = %+v", observation)
	}
}

func contentResponse(content string) map[string]any {
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}}
}

func toolCallResponse(id string, name string, args map[string]any) map[string]any {
	rawArgs, _ := json.Marshal(args)
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": nil,
		"tool_calls": []any{map[string]any{
			"id":       id,
			"function": map[string]any{"name": name, "arguments": string(rawArgs)},
		}},
	}}}}
}

type testToolCall struct {
	id   string
	name string
	args map[string]any
}

func multiToolCallResponse(calls ...testToolCall) map[string]any {
	toolCalls := make([]any, 0, len(calls))
	for _, call := range calls {
		rawArgs, _ := json.Marshal(call.args)
		toolCalls = append(toolCalls, map[string]any{
			"id":       call.id,
			"function": map[string]any{"name": call.name, "arguments": string(rawArgs)},
		})
	}
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content":    nil,
		"tool_calls": toolCalls,
	}}}}
}

func writeAgentSubagent(t *testing.T, root string, name string, profile string, toolList []string) {
	t.Helper()
	dir := filepath.Join(root, ".madharness-mini", "subagents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	rawTools, err := json.Marshal(toolList)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join([]string{
		"---",
		"name: " + name,
		"description: Test subagent " + name + ".",
		"profile: " + profile,
		"tools: " + string(rawTools),
		"max_turns: 4",
		"---",
		"",
		"Ты тестовый субагент.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsTool(schemas []map[string]any, name string) bool {
	for _, item := range schemaToolNames(schemas) {
		if item == name {
			return true
		}
	}
	return false
}

func schemaToolNames(schemas []map[string]any) []string {
	names := []string{}
	for _, schema := range schemas {
		function, _ := schema["function"].(map[string]any)
		if name, _ := function["name"].(string); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func firstEvent(events []map[string]any, name string) map[string]any {
	for _, event := range events {
		if event["event"] == name {
			return event
		}
	}
	return nil
}

func delegateObservation(t *testing.T, events []map[string]any) map[string]any {
	t.Helper()
	return toolObservation(events, "delegate_task")
}

func toolObservation(events []map[string]any, tool string) map[string]any {
	for _, event := range events {
		if event["event"] == "tool_observation" && event["tool"] == tool {
			if observation, ok := event["observation"].(map[string]any); ok {
				return observation
			}
		}
	}
	return nil
}
