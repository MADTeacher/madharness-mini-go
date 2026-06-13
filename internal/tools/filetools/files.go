// Package filetools содержит файловые инструменты workspace.
package filetools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// Specs возвращает list/read/write file tools.
func Specs() []tools.Spec {
	return []tools.Spec{listFilesSpec(), readFileSpec(), writeFileSpec()}
}

func listFiles(ctx *tools.Context, args map[string]any) tools.Observation {
	base, err := ctx.Policy.SafePath(tools.StringArg(args, "path", "."))
	if err != nil {
		return tools.Fail("list_files", err.Error())
	}
	pattern := tools.StringArg(args, "glob", "*")
	source := base
	info, err := os.Stat(source)
	if err != nil {
		source = ctx.Config.Root
		info, _ = os.Stat(source)
	}
	if info != nil && !info.IsDir() {
		results := []string{}
		if ok, _ := filepath.Match(pattern, filepath.Base(source)); ok && !tools.Ignored(source) {
			if rel, err := filepath.Rel(ctx.Config.Root, source); err == nil {
				slashed := filepath.ToSlash(rel)
				if _, err := ctx.Policy.SafePath(slashed); err == nil {
					results = append(results, slashed)
				}
			}
		}
		return tools.OK("list_files", fmt.Sprintf("listed %d files", len(results)), map[string]any{
			"files":     results,
			"truncated": false,
		})
	}
	results := []string{}
	_ = filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil || tools.Ignored(path) {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); !ok {
			return nil
		}
		rel, err := filepath.Rel(ctx.Config.Root, path)
		if err == nil {
			slashed := filepath.ToSlash(rel)
			if _, err := ctx.Policy.SafePath(slashed); err == nil {
				results = append(results, slashed)
			}
		}
		return nil
	})
	sort.Strings(results)
	truncated := false
	if len(results) > 200 {
		results = results[:200]
		truncated = true
	}
	return tools.OK("list_files", fmt.Sprintf("listed %d files", len(results)), map[string]any{
		"files":     results,
		"truncated": truncated,
	})
}

func readFile(ctx *tools.Context, args map[string]any) tools.Observation {
	rawPath := tools.StringArg(args, "path", "")
	path, err := ctx.Policy.SafePath(rawPath)
	if err != nil {
		return tools.Fail("read_file", err.Error())
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return tools.Fail("read_file", "not a file: "+rawPath)
	}
	if ctx.Trace != nil && ctx.ResourceTracker != nil {
		if event := ctx.ResourceTracker.ResourceEvent(path); event != nil {
			_ = ctx.Trace.Write("skill_resource_used", mergeEvent(event, map[string]any{"tool": "read_file"}))
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return tools.Fail("read_file", err.Error())
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	start := max(tools.IntArg(args, "start", 1), 1)
	end := min(tools.IntArg(args, "end", start+160), len(lines))
	excerpt := []string{}
	for i := start; i <= end; i++ {
		excerpt = append(excerpt, fmt.Sprintf("%d: %s", i, lines[i-1]))
	}
	return tools.OK("read_file", fmt.Sprintf("read %s:%d-%d", rawPath, start, end), map[string]any{
		"content": tools.Clipped(strings.Join(excerpt, "\n"), tools.MaxOutput),
		"start":   start,
		"end":     end,
	})
}

func writeFile(ctx *tools.Context, args map[string]any) tools.Observation {
	rawPath := tools.StringArg(args, "path", "")
	if scopeError := ctx.WritePathError(rawPath); scopeError != "" {
		return tools.Fail("write_file", scopeError)
	}
	path, err := ctx.Policy.SafePath(rawPath)
	if err != nil {
		return tools.Fail("write_file", err.Error())
	}
	content := tools.StringArg(args, "content", "")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return tools.Fail("write_file", err.Error())
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return tools.Fail("write_file", err.Error())
	}
	return tools.OK("write_file", "wrote "+rawPath, map[string]any{"bytes": len([]byte(content))})
}

func mergeEvent(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}
