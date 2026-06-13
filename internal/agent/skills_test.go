package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExplicitMarkerActivatesSkillBeforeFirstModelCall(t *testing.T) {
	cfg := testAgentConfig(t)
	writeAgentSkill(t, cfg.Root, "docs-writer")
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, tracePath, err := runWithClient("@skill:docs-writer обнови README", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" || len(client.seen) != 1 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	system := client.seen[0][0]["content"].(string)
	if !strings.Contains(system, "# Active Agent Skill: docs-writer") ||
		strings.Contains(system, "# Available Agent Skills") {
		t.Fatalf("system = %s", system)
	}
	if toolNames(client.toolsSeen[0])["activate_skill"] {
		t.Fatalf("activate_skill should be absent: %+v", client.toolsSeen[0])
	}
	if !traceHasEvent(readTraceEvents(t, tracePath), "skill_activated", "trigger", "explicit") {
		t.Fatalf("skill_activated explicit event not found")
	}
}

func TestRunModelCanActivateSkillFromCatalog(t *testing.T) {
	cfg := testAgentConfig(t)
	writeAgentSkill(t, cfg.Root, "docs-writer")
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "call_skill",
				"function": map[string]any{"name": "activate_skill", "arguments": `{"name":"docs-writer"}`},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, tracePath, err := runWithClient("обнови README", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	firstSystem := client.seen[0][0]["content"].(string)
	secondSystem := client.seen[1][0]["content"].(string)
	if !strings.Contains(firstSystem, "# Available Agent Skills") ||
		!strings.Contains(secondSystem, "# Active Agent Skill: docs-writer") {
		t.Fatalf("systems = %q / %q", firstSystem, secondSystem)
	}
	if !activateSchemaHasName(client.toolsSeen[0], "docs-writer") {
		t.Fatalf("activate schema = %+v", client.toolsSeen[0])
	}
	if strings.Contains(readFileText(t, tracePath), "Пиши коротко и проверяемо.") {
		t.Fatal("trace should not duplicate full skill body")
	}
}

func TestRunUnknownExplicitSkillFailsBeforeModelCall(t *testing.T) {
	cfg := testAgentConfig(t)
	client := &fakeClient{response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "nope"}}}}}

	_, _, err := runWithClient("@skill:docs-writer обнови README", cfg, client)

	if err == nil || !strings.Contains(err.Error(), "unknown skill: docs-writer") {
		t.Fatalf("err = %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("model was called %d times", client.calls)
	}
}

func TestAskDoesNotLoadSkillCatalogOrParseMarkers(t *testing.T) {
	cfg := testAgentConfig(t)
	writeAgentSkill(t, cfg.Root, "docs-writer")
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{"content": "ok"}}}},
	}}

	result, _, err := askWithClient("@skill:docs-writer обычный вопрос", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	rendered, _ := json.Marshal(client.seen[0])
	if result != "ok" ||
		strings.Contains(string(rendered), "Available Agent Skills") ||
		strings.Contains(string(rendered), "Active Agent Skill") {
		t.Fatalf("result=%q messages=%s", result, rendered)
	}
}

func writeAgentSkill(t *testing.T, root string, name string) {
	t.Helper()
	skillRoot := filepath.Join(root, ".agents", "skills", name)
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	text := strings.Join([]string{
		"---",
		"name: " + name,
		"description: Помогает обновлять документацию.",
		"---",
		"",
		"Пиши коротко и проверяемо.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func toolNames(schemas []map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, schema := range schemas {
		fn := schema["function"].(map[string]any)
		out[fn["name"].(string)] = true
	}
	return out
}

func activateSchemaHasName(schemas []map[string]any, name string) bool {
	for _, schema := range schemas {
		fn := schema["function"].(map[string]any)
		if fn["name"] != "activate_skill" {
			continue
		}
		params := fn["parameters"].(map[string]any)
		props := params["properties"].(map[string]any)
		nameProp := props["name"].(map[string]any)
		for _, item := range nameProp["enum"].([]any) {
			if item == name {
				return true
			}
		}
	}
	return false
}

func traceHasEvent(events []map[string]any, eventName string, key string, value string) bool {
	for _, event := range events {
		if event["event"] == eventName && event[key] == value {
			return true
		}
	}
	return false
}
