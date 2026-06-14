package turnexec

// Group хранит непрерывный участок turn-а: read batch или один barrier.
type Group struct {
	Tasks    []Task
	Parallel bool
}

// Plan группирует соседние read-only calls и оставляет остальные calls барьерами.
func Plan(tasks []Task) []Group {
	groups := []Group{}
	readBatch := []Task{}
	flushReads := func() {
		if len(readBatch) == 0 {
			return
		}
		groups = append(groups, Group{Tasks: append([]Task{}, readBatch...), Parallel: true})
		readBatch = nil
	}
	for _, task := range tasks {
		if task.readOnly() {
			readBatch = append(readBatch, task)
			continue
		}
		flushReads()
		groups = append(groups, Group{Tasks: []Task{task}})
	}
	flushReads()
	return groups
}
