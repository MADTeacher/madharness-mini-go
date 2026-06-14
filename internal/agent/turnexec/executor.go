package turnexec

import "sync"

// Execute запускает группы по порядку, а read/delegate batches - конкурентно с лимитами.
func Execute(groups []Group, maxParallelTools int, maxParallelSubagents int, handler Handler) []Result {
	results, _ := ExecuteUntil(groups, maxParallelTools, maxParallelSubagents, handler, nil)
	return results
}

// ExecuteUntil запускает группы по порядку и останавливается после result,
// который владелец loop-а считает терминальным для текущего turn-а.
func ExecuteUntil(
	groups []Group,
	maxParallelTools int,
	maxParallelSubagents int,
	handler Handler,
	shouldStop func(Result) bool,
) ([]Result, bool) {
	maxParallelTools = normalizedLimit(maxParallelTools)
	maxParallelSubagents = normalizedLimit(maxParallelSubagents)
	results := []Result{}
	for _, group := range groups {
		limit := groupLimit(group, maxParallelTools, maxParallelSubagents)
		groupResults := []Result{}
		if group.Parallel && len(group.Tasks) > 1 && limit > 1 {
			groupResults = executeParallel(group.Tasks, limit, handler)
		} else {
			for _, task := range group.Tasks {
				groupResults = append(groupResults, executeOne(task, handler))
				if shouldStop != nil && shouldStop(groupResults[len(groupResults)-1]) {
					results = append(results, groupResults...)
					return results, true
				}
			}
		}
		results = append(results, groupResults...)
		if shouldStop != nil {
			for _, result := range groupResults {
				if shouldStop(result) {
					return results, true
				}
			}
		}
	}
	return results, false
}

func normalizedLimit(value int) int {
	if value < 1 {
		return 1
	}
	return value
}

func groupLimit(group Group, maxParallelTools int, maxParallelSubagents int) int {
	switch group.Kind {
	case GroupDelegate:
		return maxParallelSubagents
	default:
		return maxParallelTools
	}
}

func executeParallel(tasks []Task, maxParallel int, handler Handler) []Result {
	results := make([]Result, len(tasks))
	limit := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for index, task := range tasks {
		wg.Add(1)
		go func(index int, task Task) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			results[index] = executeOne(task, handler)
		}(index, task)
	}
	wg.Wait()
	return results
}

func executeOne(task Task, handler Handler) Result {
	if !task.Runnable {
		return Result{Task: task, Observation: task.Observation, Followups: task.Followups}
	}
	obs, followups := handler(task)
	return Result{Task: task, Observation: obs, Followups: followups}
}
