package policy

import "fmt"

// ParsedCommand хранит argv и shell control operators, найденные вне кавычек.
type ParsedCommand struct {
	Args             []string
	ControlOperators []string
}

// SplitCommand разбирает одну команду без shell: кавычки нужны только для argv.
func SplitCommand(command string) ([]string, error) {
	parsed, err := ParseCommand(command)
	if err != nil {
		return nil, err
	}
	return parsed.Args, nil
}

// ParseCommand строит argv и отдельно отмечает control operators вне кавычек.
func ParseCommand(command string) (ParsedCommand, error) {
	parsed := ParsedCommand{}
	current := []rune{}
	runes := []rune(command)
	var quote rune
	escaped := false
	inArg := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case escaped:
			current = append(current, r)
			escaped = false
			inArg = true
		case r == '\\':
			escaped = true
			inArg = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current = append(current, r)
			}
			inArg = true
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case isShellSpace(r):
			flushArg(&parsed, &current, &inArg)
		case isControlRune(r):
			flushArg(&parsed, &current, &inArg)
			operator := string(r)
			if i+1 < len(runes) && isDoubleControlOperator(r, runes[i+1]) {
				operator += string(runes[i+1])
				i++
			}
			parsed.ControlOperators = append(parsed.ControlOperators, operator)
		default:
			current = append(current, r)
			inArg = true
		}
	}
	if escaped {
		current = append(current, '\\')
	}
	if quote != 0 {
		return ParsedCommand{}, fmt.Errorf("unterminated quote")
	}
	flushArg(&parsed, &current, &inArg)
	return parsed, nil
}

// HasShellControlOperator проверяет shell grammar tokens вне кавычек.
func HasShellControlOperator(command string) bool {
	parsed, err := ParseCommand(command)
	if err != nil {
		return false
	}
	return len(parsed.ControlOperators) > 0
}

func flushArg(parsed *ParsedCommand, current *[]rune, inArg *bool) {
	if !*inArg {
		return
	}
	parsed.Args = append(parsed.Args, string(*current))
	*current = (*current)[:0]
	*inArg = false
}

func isShellSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isControlRune(r rune) bool {
	return r == '|' || r == '&' || r == ';' || r == '>' || r == '<'
}

func isDoubleControlOperator(first rune, second rune) bool {
	return (first == '&' && second == '&') || (first == '|' && second == '|')
}
