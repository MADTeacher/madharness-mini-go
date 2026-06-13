package patchtool

import "fmt"

func applyHunk(current []string, oldLines []string, newLines []string) ([]string, error) {
	if len(oldLines) == 0 {
		return nil, fmt.Errorf("update hunk must include context or removed lines")
	}
	matches := []int{}
	for start := 0; start <= len(current)-len(oldLines); start++ {
		if sameLines(current[start:start+len(oldLines)], oldLines) {
			matches = append(matches, start)
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("expected 1 hunk match, found %d", len(matches))
	}
	start := matches[0]
	end := start + len(oldLines)
	next := append([]string{}, current[:start]...)
	next = append(next, newLines...)
	next = append(next, current[end:]...)
	return next, nil
}

func sameLines(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
