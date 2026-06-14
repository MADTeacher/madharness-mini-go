// Package patchtool реализует строгий Codex-style apply_patch.
package patchtool

import "github.com/MADTeacher/madharness-mini-go/internal/tools"

// Spec возвращает apply_patch tool.
func Spec() tools.Spec {
	return tools.Spec{
		Name:        "apply_patch",
		Description: applyPatchDescription,
		Parameters: tools.Obj(map[string]any{
			"patch": tools.StrParam("", patchArgumentDescription, true),
		}, []string{"patch"}),
		Handler: applyPatch,
		Effect:  tools.EffectWrite,
	}
}

const applyPatchDescription = `Apply a strict Codex-style patch inside the workspace.

Use this for precise edits to existing files, file creation, deletion, and moves.
The patch argument is one multiline string, not a shell command or JSON object.
The parser is strict: keep markers exactly as shown and include enough context
for each update hunk to match exactly one place.
If apply_patch fails, use read_file or search_code to get exact current file text
and retry with verbatim context. Do not switch to write_file or run_shell scripts
for precise edits.`

const patchArgumentDescription = `Strict Codex-style patch text.

Required shape:
*** Begin Patch
*** Update File: path
@@
 context line begins with one space
-removed line begins with minus
+added line begins with plus
*** End Patch

Supported file operations:
*** Add File: path       then every content line must start with +
*** Update File: path    then one or more @@ hunks, or optional Move to
*** Delete File: path
*** Move to: path        only immediately after Update File

On failure: reread the current file region with read_file/search_code, copy exact
current lines into the hunk, and retry apply_patch once.`
