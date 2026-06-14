package turnexec

import "sync"

// Execute запускает группы по порядку, а read/delegate batches - конкурентно с лимитами.
func Execute(groups []Group, maxParallelTools int, maxParallelSubagents int, handler Handler) []Result {
	maxParallelTools = normalizedLimit(maxParallelTools)
	maxParallelSubagents = normalizedLimit(maxParallelSubagents)
	results := []Result{}
	for _, group := range groups {
		limit := groupLimit(group, maxParallelTools, maxParallelSubagents)
		if group.Parallel && len(group.Tasks) > 1 && limit > 1 {
			results = append(results, executeParallel(group.Tasks, limit, handler)...)
			continue
		}
		for _, task := range group.Tasks {
			results = append(results, executeOne(task, handler))
		}
	}
	return results
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
