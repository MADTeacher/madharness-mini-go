package policy

import "fmt"

// SplitCommand разбирает одну команду без shell: кавычки нужны только для argv.
func SplitCommand(command string) ([]string, error) {
	args := []string{}
	current := []rune{}
	var quote rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			current = append(current, r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current = append(current, r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if len(current) > 0 {
				args = append(args, string(current))
				current = current[:0]
			}
		default:
			current = append(current, r)
		}
	}
	if escaped {
		current = append(current, '\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if len(current) > 0 {
		args = append(args, string(current))
	}
	return args, nil
}
