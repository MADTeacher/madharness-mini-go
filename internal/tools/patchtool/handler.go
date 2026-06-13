package patchtool

import (
	"os"
	"path/filepath"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func applyPatch(ctx *tools.Context, args map[string]any) tools.Observation {
	parser := Parser{ctx: ctx}
	changes, err := parser.Prepare(tools.StringArg(args, "patch", ""))
	if err != nil {
		summary := err.Error()
		return tools.Fail("apply_patch", summary, failureData(summary))
	}
	for _, change := range changes {
		if change.Content == nil {
			if err := os.Remove(change.Path); err != nil {
				return tools.Fail("apply_patch", err.Error())
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(change.Path), 0o755); err != nil {
			return tools.Fail("apply_patch", err.Error())
		}
		if err := os.WriteFile(change.Path, []byte(*change.Content), 0o644); err != nil {
			return tools.Fail("apply_patch", err.Error())
		}
	}
	return tools.OK("apply_patch", "applied patch to "+itoa(len(uniqueChangePaths(changes)))+" file(s)", nil)
}

func uniqueChangePaths(changes []Change) map[string]bool {
	paths := map[string]bool{}
	for _, change := range changes {
		paths[change.Path] = true
	}
	return paths
}
