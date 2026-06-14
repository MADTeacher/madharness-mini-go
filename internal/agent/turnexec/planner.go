package turnexec

const (
	GroupBarrier  = "barrier"
	GroupRead     = "read"
	GroupMCP      = "mcp"
	GroupDelegate = "delegate"
)

// Group хранит непрерывный участок turn-а: read/MCP/delegate batch или один barrier.
type Group struct {
	Tasks    []Task
	Parallel bool
	Kind     string
}

// Plan группирует соседние read-only/MCP/delegate calls и оставляет остальные calls барьерами.
func Plan(tasks []Task) []Group {
	groups := []Group{}
	var batch []Task
	batchKind := ""
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		groups = append(groups, Group{Tasks: append([]Task{}, batch...), Parallel: true, Kind: batchKind})
		batch = nil
		batchKind = ""
	}
	addBatch := func(kind string, task Task) {
		if batchKind != "" && batchKind != kind {
			flushBatch()
		}
		batchKind = kind
		batch = append(batch, task)
	}
	for _, task := range tasks {
		if task.readOnly() {
			addBatch(GroupRead, task)
			continue
		}
		if task.mcp() {
			addBatch(GroupMCP, task)
			continue
		}
		if task.delegate() {
			addBatch(GroupDelegate, task)
			continue
		}
		flushBatch()
		groups = append(groups, Group{Tasks: []Task{task}, Kind: GroupBarrier})
	}
	flushBatch()
	return groups
}
