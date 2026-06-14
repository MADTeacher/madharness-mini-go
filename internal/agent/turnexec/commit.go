package turnexec

// Committer применяет result к владельцу agent state.
type Committer func(Result) bool

// Commit применяет results строго в порядке, который вернул Execute.
func Commit(results []Result, commit Committer) bool {
	for _, result := range results {
		if commit(result) {
			return true
		}
	}
	return false
}
