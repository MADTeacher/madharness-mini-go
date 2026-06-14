package turnexec

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestExecuteRunsReadBatchConcurrentlyAndKeepsOrder(t *testing.T) {
	tasks := []Task{
		{Index: 0, Name: "first", Effect: tools.EffectRead, Runnable: true},
		{Index: 1, Name: "second", Effect: tools.EffectRead, Runnable: true},
	}
	started := make(chan string, len(tasks))
	release := make(chan struct{})
	done := make(chan []Result, 1)

	go func() {
		groups := Plan(tasks)
		done <- Execute(groups, 2, 1, func(task Task) (tools.Observation, []map[string]any) {
			started <- task.Name
			<-release
			return tools.OK(task.Name, task.Name+" done", nil), nil
		})
	}()

	waitStarted(t, started)
	waitStarted(t, started)
	close(release)

	select {
	case results := <-done:
		if len(results) != 2 || results[0].Task.Name != "first" || results[1].Task.Name != "second" {
			t.Fatalf("results order = %+v", results)
		}
	case <-time.After(time.Second):
		t.Fatal("parallel read batch did not finish")
	}
}

func TestPlanKeepsExclusiveTasksAsBarriers(t *testing.T) {
	groups := Plan([]Task{
		{Index: 0, Name: "read_file", Effect: tools.EffectRead, Runnable: true},
		{Index: 1, Name: "write_file", Effect: tools.EffectWrite, Runnable: true},
		{Index: 2, Name: "search_code", Effect: tools.EffectRead, Runnable: true},
		{Index: 3, Name: "blocked", Effect: tools.EffectRead, Runnable: false},
	})

	if len(groups) != 4 {
		t.Fatalf("groups = %+v", groups)
	}
	if !groups[0].Parallel || groups[1].Parallel || !groups[2].Parallel || groups[3].Parallel {
		t.Fatalf("parallel flags = %+v", groups)
	}
	if groups[1].Tasks[0].Name != "write_file" || groups[3].Tasks[0].Name != "blocked" {
		t.Fatalf("barriers = %+v", groups)
	}
}

func TestPlanSeparatesReadAndDelegateBatches(t *testing.T) {
	groups := Plan([]Task{
		{Index: 0, Name: "read_file", Effect: tools.EffectRead, Runnable: true},
		{Index: 1, Name: "delegate_task", Effect: tools.EffectDelegate, Runnable: true},
		{Index: 2, Name: "delegate_task", Effect: tools.EffectDelegate, Runnable: true},
		{Index: 3, Name: "search_code", Effect: tools.EffectRead, Runnable: true},
	})

	if len(groups) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Kind != GroupRead || groups[1].Kind != GroupDelegate || groups[2].Kind != GroupRead {
		t.Fatalf("group kinds = %+v", groups)
	}
	if !groups[1].Parallel || len(groups[1].Tasks) != 2 {
		t.Fatalf("delegate batch = %+v", groups[1])
	}
}

func TestPlanSeparatesMCPBatches(t *testing.T) {
	groups := Plan([]Task{
		{Index: 0, Name: "read_file", Effect: tools.EffectRead, Runnable: true},
		{Index: 1, Name: "mcp__fake__first", Effect: tools.EffectMCP, Runnable: true},
		{Index: 2, Name: "mcp__fake__second", Effect: tools.EffectMCP, Runnable: true},
		{Index: 3, Name: "delegate_task", Effect: tools.EffectDelegate, Runnable: true},
	})

	if len(groups) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Kind != GroupRead || groups[1].Kind != GroupMCP || groups[2].Kind != GroupDelegate {
		t.Fatalf("group kinds = %+v", groups)
	}
	if !groups[1].Parallel || len(groups[1].Tasks) != 2 {
		t.Fatalf("MCP batch = %+v", groups[1])
	}
}

func TestExecuteUsesToolLimitForMCPBatch(t *testing.T) {
	tasks := []Task{
		{Index: 0, Name: "first", Effect: tools.EffectMCP, Runnable: true},
		{Index: 1, Name: "second", Effect: tools.EffectMCP, Runnable: true},
	}
	started := make(chan string, len(tasks))
	release := make(chan struct{})
	released := atomic.Bool{}
	earlyStart := atomic.Bool{}
	done := make(chan []Result, 1)

	go func() {
		done <- Execute(Plan(tasks), 1, 2, func(task Task) (tools.Observation, []map[string]any) {
			if task.Name == "second" && !released.Load() {
				earlyStart.Store(true)
			}
			started <- task.Name
			<-release
			return tools.OK(task.Name, task.Name+" done", nil), nil
		})
	}()

	waitStarted(t, started)
	released.Store(true)
	close(release)
	waitStarted(t, started)
	assertNoEarlyStart(t, &earlyStart, "second")

	select {
	case results := <-done:
		if len(results) != 2 || results[0].Task.Name != "first" || results[1].Task.Name != "second" {
			t.Fatalf("results order = %+v", results)
		}
	case <-time.After(time.Second):
		t.Fatal("MCP batch did not finish")
	}
}

func TestExecuteUsesSubagentLimitForDelegateBatch(t *testing.T) {
	tasks := []Task{
		{Index: 0, Name: "first", Effect: tools.EffectDelegate, Runnable: true},
		{Index: 1, Name: "second", Effect: tools.EffectDelegate, Runnable: true},
	}
	started := make(chan string, len(tasks))
	release := make(chan struct{})
	released := atomic.Bool{}
	earlyStart := atomic.Bool{}
	done := make(chan []Result, 1)

	go func() {
		done <- Execute(Plan(tasks), 2, 1, func(task Task) (tools.Observation, []map[string]any) {
			if task.Name == "second" && !released.Load() {
				earlyStart.Store(true)
			}
			started <- task.Name
			<-release
			return tools.OK(task.Name, task.Name+" done", nil), nil
		})
	}()

	waitStarted(t, started)
	released.Store(true)
	close(release)
	waitStarted(t, started)
	assertNoEarlyStart(t, &earlyStart, "second")

	select {
	case results := <-done:
		if len(results) != 2 || results[0].Task.Name != "first" || results[1].Task.Name != "second" {
			t.Fatalf("results order = %+v", results)
		}
	case <-time.After(time.Second):
		t.Fatal("delegate batch did not finish")
	}
}

func TestExecuteUntilStopsBeforeLaterBarrier(t *testing.T) {
	tasks := []Task{
		{Index: 0, Name: "ask_user", Effect: tools.EffectState, Runnable: true},
		{Index: 1, Name: "write_file", Effect: tools.EffectWrite, Runnable: true},
	}
	called := []string{}

	results, stopped := ExecuteUntil(Plan(tasks), 1, 1, func(task Task) (tools.Observation, []map[string]any) {
		called = append(called, task.Name)
		obs := tools.OK(task.Name, "done", nil)
		if task.Name == "ask_user" {
			obs["_subagent_stop"] = "needs_user_input"
		}
		return obs, nil
	}, func(result Result) bool {
		value, _ := result.Observation["_subagent_stop"].(string)
		return value == "needs_user_input"
	})

	if !stopped {
		t.Fatal("expected execution to stop")
	}
	if strings.Join(called, ",") != "ask_user" {
		t.Fatalf("called = %v", called)
	}
	if len(results) != 1 || results[0].Task.Name != "ask_user" {
		t.Fatalf("results = %+v", results)
	}
}

func waitStarted(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}
}

func assertNoEarlyStart(t *testing.T, earlyStart *atomic.Bool, name string) {
	t.Helper()
	if earlyStart.Load() {
		t.Fatalf("%s started before the first task released its slot", name)
	}
}
