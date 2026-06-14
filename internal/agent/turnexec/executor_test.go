package turnexec

import (
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
		done <- Execute(groups, 2, func(task Task) (tools.Observation, []map[string]any) {
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

func waitStarted(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}
}
