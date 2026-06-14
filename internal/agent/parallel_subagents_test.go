package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestParallelDelegateTasksOverlapWhenEnabled(t *testing.T) {
	cfg := testAgentConfig(t)
	client := newParallelDelegateClient()
	done := runParallelDelegateScenario(t, cfg, client, 2)

	waitToolsStarted(t, client.started, "first child", "second child")
	close(client.firstRelease)
	close(client.secondRelease)
	result := waitParallelDelegateDone(t, done)

	if result.result != "parent done" {
		t.Fatalf("result = %q", result.result)
	}
	observations := delegateObservations(readTraceEvents(t, result.tracePath))
	if len(observations) != 2 {
		t.Fatalf("delegate observations = %+v", observations)
	}
	if observations[0]["answer"] != "first child done" || observations[1]["answer"] != "second child done" {
		t.Fatalf("delegate observation order = %+v", observations)
	}
}

func TestParallelDelegateTasksRespectSubagentLimitOne(t *testing.T) {
	cfg := testAgentConfig(t)
	client := newParallelDelegateClient()
	done := runParallelDelegateScenario(t, cfg, client, 1)

	waitToolStarted(t, client.started, "first child")
	assertNoToolStart(t, client.started)
	close(client.firstRelease)
	waitToolStarted(t, client.started, "second child")
	close(client.secondRelease)
	result := waitParallelDelegateDone(t, done)

	if result.result != "parent done" {
		t.Fatalf("result = %q", result.result)
	}
}

func TestChildSessionDoesNotCopyParentHistory(t *testing.T) {
	cfg := testAgentConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "note.txt"), []byte("SECRET_PARENT_CONTEXT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &childContextClient{}

	result, _, err := runWithClientOptions("read then delegate", cfg, client, RunOptions{OrchestrationMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "parent done" {
		t.Fatalf("result = %q", result)
	}
	childText := client.childMessagesText()
	if strings.Contains(childText, "SECRET_PARENT_CONTEXT") {
		t.Fatalf("child context copied parent history: %s", childText)
	}
	if !strings.Contains(childText, "allowed child context") {
		t.Fatalf("child context did not include explicit delegate context: %s", childText)
	}
}

type parallelDelegateRunResult struct {
	result    string
	tracePath string
	err       error
}

type parallelDelegateClient struct {
	mu            sync.Mutex
	parentCalls   int
	started       chan string
	firstRelease  chan struct{}
	secondRelease chan struct{}
}

func newParallelDelegateClient() *parallelDelegateClient {
	return &parallelDelegateClient{
		started:       make(chan string, 2),
		firstRelease:  make(chan struct{}),
		secondRelease: make(chan struct{}),
	}
}

func runParallelDelegateScenario(t *testing.T, cfg *config.Config, client *parallelDelegateClient, maxSubagents int) <-chan parallelDelegateRunResult {
	t.Helper()
	done := make(chan parallelDelegateRunResult, 1)
	go func() {
		result, tracePath, err := runWithClientOptions("delegate twice", cfg, client, RunOptions{
			OrchestrationMode:    "auto",
			MaxParallelSubagents: maxSubagents,
		})
		done <- parallelDelegateRunResult{result: result, tracePath: tracePath, err: err}
	}()
	return done
}

func (c *parallelDelegateClient) Chat(messages []map[string]any, tools []map[string]any) (map[string]any, error) {
	text := messagesText(messages)
	if !containsTool(tools, "delegate_task") {
		if strings.Contains(text, "first child") {
			c.started <- "first child"
			<-c.firstRelease
			return contentResponse("first child done"), nil
		}
		if strings.Contains(text, "second child") {
			c.started <- "second child"
			<-c.secondRelease
			return contentResponse("second child done"), nil
		}
		return contentResponse("child done"), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parentCalls++
	if c.parentCalls == 1 {
		return twoDelegateCallsResponse(), nil
	}
	return contentResponse("parent done"), nil
}

type childContextClient struct {
	mu            sync.Mutex
	parentCalls   int
	childMessages []map[string]any
}

func (c *childContextClient) Chat(messages []map[string]any, tools []map[string]any) (map[string]any, error) {
	if !containsTool(tools, "delegate_task") {
		c.mu.Lock()
		c.childMessages = copyMessagesForTest(messages)
		c.mu.Unlock()
		return contentResponse("child done"), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parentCalls++
	if c.parentCalls == 1 {
		return toolCallResponse("call_read", "read_file", map[string]any{"path": "note.txt"}), nil
	}
	if c.parentCalls == 2 {
		return toolCallResponse("call_delegate", "delegate_task", map[string]any{
			"subagent": "researcher",
			"task":     "child task",
			"context":  "allowed child context",
		}), nil
	}
	return contentResponse("parent done"), nil
}

func (c *childContextClient) childMessagesText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return messagesText(c.childMessages)
}

func twoDelegateCallsResponse() map[string]any {
	firstArgs, _ := json.Marshal(map[string]any{"subagent": "researcher", "task": "first child"})
	secondArgs, _ := json.Marshal(map[string]any{"subagent": "researcher", "task": "second child"})
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": nil,
		"tool_calls": []any{
			map[string]any{
				"id":       "call_first",
				"function": map[string]any{"name": "delegate_task", "arguments": string(firstArgs)},
			},
			map[string]any{
				"id":       "call_second",
				"function": map[string]any{"name": "delegate_task", "arguments": string(secondArgs)},
			},
		},
	}}}}
}

func waitParallelDelegateDone(t *testing.T, done <-chan parallelDelegateRunResult) parallelDelegateRunResult {
	t.Helper()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result
	case <-time.After(time.Second):
		t.Fatal("parallel delegate run did not finish")
		return parallelDelegateRunResult{}
	}
}

func delegateObservations(events []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, event := range events {
		if event["event"] == "tool_observation" && event["tool"] == "delegate_task" {
			if observation, ok := event["observation"].(map[string]any); ok {
				out = append(out, observation)
			}
		}
	}
	return out
}

func messagesText(messages []map[string]any) string {
	raw, _ := json.Marshal(messages)
	return string(raw)
}

func copyMessagesForTest(messages []map[string]any) []map[string]any {
	raw, _ := json.Marshal(messages)
	out := []map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}
