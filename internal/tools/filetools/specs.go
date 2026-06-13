package filetools

import "github.com/MADTeacher/madharness-mini-go/internal/tools"

func listFilesSpec() tools.Spec {
	return tools.Spec{
		Name:        "list_files",
		Description: listFilesDescription,
		Parameters: tools.Obj(map[string]any{
			"path": tools.StrParam(".", "Workspace-relative directory or file to inspect; defaults to .", false),
			"glob": tools.StrParam("*", "fnmatch-style pattern matched against file names only; defaults to *", false),
		}, nil),
		Handler: listFiles,
	}
}

func readFileSpec() tools.Spec {
	return tools.Spec{
		Name:        "read_file",
		Description: readFileDescription,
		Parameters: tools.Obj(map[string]any{
			"path":  tools.StrParam("", "Workspace-relative file path to read.", true),
			"start": map[string]any{"type": "integer", "default": 1, "description": "1-based first line number to include; defaults to 1."},
			"end":   map[string]any{"type": "integer", "default": 160, "description": "1-based last line number to include; defaults to start + 160."},
		}, []string{"path"}),
		Handler: readFile,
	}
}

func writeFileSpec() tools.Spec {
	return tools.Spec{
		Name:        "write_file",
		Description: writeFileDescription,
		Parameters: tools.Obj(map[string]any{
			"path":    tools.StrParam("", "Workspace-relative file path to create or intentionally fully overwrite.", true),
			"content": tools.StrParam("", "Complete UTF-8 file content to write, including final newline if wanted.", true),
		}, []string{"path", "content"}),
		Handler: writeFile,
	}
}

const listFilesDescription = `Recursively list files inside the workspace.

Use this to discover repository structure before reading files. Results include
files only, skip ignored folders such as .git and caches, and stop after 200
matches. The glob filter matches each file name, not the full relative path.`

const readFileDescription = `Read a UTF-8 file excerpt from the workspace.

Use this before editing a file. start and end are 1-based line numbers, and the
returned content includes numbered lines like "12: text" so later edits can
refer to the exact surrounding text.`

const writeFileDescription = `Write a complete UTF-8 text file inside the workspace.

This creates parent directories as needed and fully overwrites the target file.
Prefer apply_patch for precise edits to existing files. Do not use write_file as
the fallback for a failed precise edit unless you intentionally need a full-file
rewrite.`
