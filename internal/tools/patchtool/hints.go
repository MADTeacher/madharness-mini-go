package patchtool

import "strings"

func failureData(summary string) map[string]any {
	switch {
	case summary == "expected 1 hunk match, found 0":
		return map[string]any{
			"hint":      "The update hunk did not match the current file. Use read_file or search_code to reread the exact region, then retry apply_patch with verbatim current context lines, including spaces.",
			"retryable": true,
		}
	case strings.HasPrefix(summary, "expected 1 hunk match, found "):
		return map[string]any{
			"hint":      "The update hunk matched more than one place. Add more surrounding context lines copied exactly from the current file, then retry apply_patch.",
			"retryable": true,
		}
	case summary == "invalid hunk line: ":
		return map[string]any{
			"hint":      "The update hunk contains a blank line without a marker. Blank context lines must still start with one leading space.",
			"retryable": true,
		}
	case syntaxFailure(summary):
		return map[string]any{
			"hint":      "Send only the patch text, starting with *** Begin Patch and ending with *** End Patch. Do not wrap it in a shell command, Markdown fence, or extra prose.",
			"retryable": true,
		}
	case summary == "update hunk must include context or removed lines":
		return map[string]any{
			"hint":      "An update hunk needs at least one current context or removed line. Use read_file to copy exact nearby lines, then retry apply_patch.",
			"retryable": true,
		}
	default:
		return nil
	}
}

func syntaxFailure(summary string) bool {
	return strings.HasPrefix(summary, "patch must ") ||
		strings.HasPrefix(summary, "unexpected patch line") ||
		strings.HasPrefix(summary, "invalid hunk line") ||
		strings.HasPrefix(summary, "add file lines must start") ||
		summary == "Move to is only supported after Update File"
}
