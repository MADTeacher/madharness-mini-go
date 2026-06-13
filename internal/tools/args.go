package tools

import (
	"encoding/json"
	"fmt"
)

// ParseToolArgs достаёт function.name и JSON arguments из tool_call модели.
func ParseToolArgs(call map[string]any) (string, map[string]any, error) {
	fn, ok := call["function"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("missing function")
	}
	name, _ := fn["name"].(string)
	raw := fn["arguments"]
	if raw == nil {
		return name, map[string]any{}, nil
	}
	if args, ok := raw.(map[string]any); ok {
		return name, args, nil
	}
	text, ok := raw.(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid arguments")
	}
	args := map[string]any{}
	if err := json.Unmarshal([]byte(text), &args); err != nil {
		return "", nil, err
	}
	return name, args, nil
}

// StringArg читает строковый аргумент tool call.
func StringArg(args map[string]any, name string, fallback string) string {
	if value, ok := args[name].(string); ok {
		return value
	}
	return fallback
}

// IntArg читает integer аргумент, учитывая float64 после json.Unmarshal.
func IntArg(args map[string]any, name string, fallback int) int {
	switch value := args[name].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}
