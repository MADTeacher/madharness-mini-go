package hooks

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

const (
	defaultStringLimit = 2000
	defaultListLimit   = 20
	defaultDepthLimit  = 5
)

var sensitiveKeys = map[string]bool{
	"apikey":        true,
	"authorization": true,
	"password":      true,
	"secret":        true,
	"token":         true,
}

// CompactPayload обрезает большие структуры и прячет очевидные секреты.
func CompactPayload(value any) any {
	return compactPayload(value, defaultStringLimit, defaultListLimit, defaultDepthLimit)
}

func compactPayload(value any, stringLimit int, listLimit int, depth int) any {
	if depth <= 0 {
		return "<max depth>"
	}
	switch item := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, nested := range item {
			if isSensitiveKey(key) {
				out[key] = "<redacted>"
				continue
			}
			out[key] = compactPayload(nested, stringLimit, listLimit, depth-1)
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for key, nested := range item {
			if isSensitiveKey(key) {
				out[key] = "<redacted>"
				continue
			}
			out[key] = compactPayload(nested, stringLimit, listLimit, depth-1)
		}
		return out
	case []any:
		out := []any{}
		for index, nested := range item {
			if index >= listLimit {
				out = append(out, fmt.Sprintf("<clipped %d items>", len(item)-listLimit))
				break
			}
			out = append(out, compactPayload(nested, stringLimit, listLimit, depth-1))
		}
		return out
	case []map[string]any:
		out := []any{}
		for index, nested := range item {
			if index >= listLimit {
				out = append(out, fmt.Sprintf("<clipped %d items>", len(item)-listLimit))
				break
			}
			out = append(out, compactPayload(nested, stringLimit, listLimit, depth-1))
		}
		return out
	case []map[string]string:
		out := []any{}
		for index, nested := range item {
			if index >= listLimit {
				out = append(out, fmt.Sprintf("<clipped %d items>", len(item)-listLimit))
				break
			}
			out = append(out, compactPayload(nested, stringLimit, listLimit, depth-1))
		}
		return out
	case []string:
		out := []any{}
		for index, nested := range item {
			if index >= listLimit {
				out = append(out, fmt.Sprintf("<clipped %d items>", len(item)-listLimit))
				break
			}
			out = append(out, compactPayload(nested, stringLimit, listLimit, depth-1))
		}
		return out
	case string:
		return tools.Clipped(item, stringLimit)
	case int, int64, float64, bool, nil:
		return item
	default:
		return tools.Clipped(fmt.Sprint(item), stringLimit)
	}
}

func isSensitiveKey(key string) bool {
	lowered := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	parts := strings.FieldsFunc(lowered, func(ch rune) bool { return ch == '_' })
	if strings.HasSuffix(lowered, "api_key") || strings.HasSuffix(lowered, "_token") {
		return true
	}
	for _, part := range parts {
		if sensitiveKeys[part] {
			return true
		}
	}
	return false
}
