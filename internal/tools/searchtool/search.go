// Package searchtool содержит инструмент буквального поиска по workspace.
package searchtool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

const searchFileMaxBytes = 1 * 1024 * 1024

// Spec возвращает search_code tool.
func Spec() tools.Spec {
	return tools.Spec{
		Name:        "search_code",
		Description: description,
		Parameters: tools.Obj(map[string]any{
			"query": tools.StrParam("", "Literal substring to find; not regex and not semantic search.", true),
			"glob":  tools.StrParam("*", "fnmatch-style pattern matched against file names only; defaults to *", false),
		}, []string{"query"}),
		Handler: searchCode,
		Effect:  tools.EffectRead,
	}
}

func searchCode(ctx *tools.Context, args map[string]any) tools.Observation {
	query := tools.StringArg(args, "query", "")
	pattern := tools.StringArg(args, "glob", "*")
	matches := []map[string]any{}
	fileTruncated := false
	_ = filepath.WalkDir(ctx.Config.Root, func(path string, d os.DirEntry, err error) error {
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
		if err != nil {
			return nil
		}
		if _, err := ctx.Policy.SafePath(filepath.ToSlash(rel)); err != nil {
			return nil
		}
		if addMatches(ctx.Config.Root, path, query, &matches) {
			fileTruncated = true
		}
		if len(matches) >= 100 {
			return filepath.SkipAll
		}
		return nil
	})
	truncated := len(matches) >= 100 || fileTruncated
	return tools.OK("search_code", fmt.Sprintf("found %d matches", len(matches)), map[string]any{
		"results":   matches,
		"truncated": truncated,
	})
}

func addMatches(root string, path string, query string, matches *[]map[string]any) bool {
	raw, _, truncated, err := tools.ReadRegularFilePrefix(path, searchFileMaxBytes)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return truncated
	}
	for no, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, query) {
			continue
		}
		preview := strings.TrimSpace(line)
		if len(preview) > 240 {
			preview = preview[:240]
		}
		*matches = append(*matches, map[string]any{
			"path":    filepath.ToSlash(rel),
			"line":    no + 1,
			"preview": preview,
		})
		if len(*matches) >= 100 {
			return truncated
		}
	}
	return truncated
}

const description = `Search for a literal substring in workspace files.

Use this to find names, snippets, and exact text before reading or editing. This
is not regex or semantic search. It scans non-ignored files, returns line
numbers with previews, filters file names with glob, and stops after 100 matches.`
