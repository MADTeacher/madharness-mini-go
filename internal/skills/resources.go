package skills

import (
	"os"
	"path/filepath"
	"strings"
)

const maxListedResources = 200

// ListResources перечисляет файлы рядом со skill без чтения их содержимого.
func ListResources(skill Skill, workspaceRoot string) []Resource {
	resources := []Resource{}
	root := filepath.Clean(skill.Root)
	resolvedWorkspace := filepath.Clean(workspaceRoot)
	if realRoot, err := filepath.EvalSymlinks(workspaceRoot); err == nil {
		resolvedWorkspace = filepath.Clean(realRoot)
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(resources) >= maxListedResources {
			return nil
		}
		if d.IsDir() || filepath.Base(path) == "SKILL.md" {
			return nil
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !pathInside(resolved, root) {
			return nil
		}
		info, err := os.Stat(resolved)
		if err != nil || info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil || relative == "." {
			return nil
		}
		workspacePath, err := filepath.Rel(resolvedWorkspace, resolved)
		if err != nil || workspacePath == ".." {
			return nil
		}
		kind := "file"
		parts := splitSlash(relative)
		if len(parts) > 0 {
			kind = parts[0]
		}
		resources = append(resources, Resource{
			RelativePath:  filepath.ToSlash(relative),
			WorkspacePath: filepath.ToSlash(workspacePath),
			Kind:          kind,
			Bytes:         info.Size(),
		})
		return nil
	})
	return resources
}

func resourceMaps(resources []Resource) []map[string]any {
	out := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		out = append(out, resource.AsMap())
	}
	return out
}

func splitSlash(path string) []string {
	parts := []string{}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func splitBySlash(path string) []string {
	return strings.Split(path, "/")
}
