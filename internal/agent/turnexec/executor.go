package turnexec

import "sync"

// Execute запускает группы по порядку, а read batch - конкурентно с лимитом.
func Execute(groups []Group, maxParallel int, handler Handler) []Result {
	if maxParallel < 1 {
		maxParallel = 1
	}
	results := []Result{}
	for _, group := range groups {
		if group.Parallel && len(group.Tasks) > 1 && maxParallel > 1 {
			results = append(results, executeParallel(group.Tasks, maxParallel, handler)...)
			continue
		}
		for _, task := range group.Tasks {
			results = append(results, executeOne(task, handler))
		}
	}
	return results
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
