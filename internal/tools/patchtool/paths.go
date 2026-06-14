package patchtool

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// TouchedPaths извлекает workspace paths из заголовков patch до применения.
func TouchedPaths(ctx *tools.Context, patch string) ([]string, error) {
	paths, _, err := touchedPaths(ctx, patch)
	return paths, err
}

func touchedPaths(ctx *tools.Context, patch string) ([]string, map[string]string, error) {
	lines := splitPatchLines(patch)
	if len(lines) == 0 || lines[0] != "*** Begin Patch" {
		return nil, nil, fmt.Errorf("patch must start with *** Begin Patch")
	}
	if lines[len(lines)-1] != "*** End Patch" {
		return nil, nil, fmt.Errorf("patch must end with *** End Patch")
	}
	paths := []string{}
	pathMap := map[string]string{}
	for _, line := range lines[1 : len(lines)-1] {
		raw := ""
		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			raw = strings.TrimPrefix(line, "*** Add File: ")
		case strings.HasPrefix(line, "*** Update File: "):
			raw = strings.TrimPrefix(line, "*** Update File: ")
		case strings.HasPrefix(line, "*** Delete File: "):
			raw = strings.TrimPrefix(line, "*** Delete File: ")
		case strings.HasPrefix(line, "*** Move to: "):
			raw = strings.TrimPrefix(line, "*** Move to: ")
		}
		if raw == "" {
			continue
		}
		if scopeError := ctx.WritePathError(raw); scopeError != "" {
			return nil, nil, fmt.Errorf("%s", scopeError)
		}
		path, err := ctx.SafePathForTool("apply_patch", raw, "patch_path")
		if err != nil {
			return nil, nil, err
		}
		paths = append(paths, path)
		pathMap[raw] = path
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("patch does not mention any file")
	}
	return paths, pathMap, nil
}
