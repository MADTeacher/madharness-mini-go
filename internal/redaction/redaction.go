// Package redaction прячет секреты в диагностических payload hooks и trace.
package redaction

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// RedactedValue заменяет значения, похожие на секреты.
	RedactedValue = "<redacted>"
)

var sensitiveKeys = map[string]bool{
	"apikey":        true,
	"authorization": true,
	"password":      true,
	"secret":        true,
	"token":         true,
}

// Регулярки находят кандидатов внутри строк; helper-ы ниже решают, похожи ли они на реальные секреты.
var (
	authorizationValuePattern = regexp.MustCompile(`(?i)\b(authorization\s*[:=]\s*(?:bearer|basic)\s+)([A-Za-z0-9._~+/\-=]{6,})`)
	bearerValuePattern        = regexp.MustCompile(`(?i)\b(bearer\s+)([A-Za-z0-9._~+/\-=]{6,})`)
	assignmentValuePattern    = regexp.MustCompile(`(?i)\b([A-Za-z0-9_-]*(?:api[_-]?key|access[_-]?token|refresh[_-]?token|auth[_-]?token|token|password|secret)[A-Za-z0-9_-]*)(\s*[:=]\s*)(["']?)([^"'\s,;#]+)(["']?)`)
)

var placeholderSecrets = map[string]bool{
	"changeme":    true,
	"demo":        true,
	"example":     true,
	"none":        true,
	"null":        true,
	"password":    true,
	"placeholder": true,
	"secret":      true,
	"token":       true,
}

// RedactPayload рекурсивно заменяет секреты, не обрезая диагностический payload.
func RedactPayload(value any) any {
	return redactPayload(value)
}

func redactPayload(value any) any {
	switch item := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, nested := range item {
			if IsSensitiveKey(key) {
				out[key] = RedactedValue
				continue
			}
			out[key] = redactPayload(nested)
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for key, nested := range item {
			if IsSensitiveKey(key) {
				out[key] = RedactedValue
				continue
			}
			out[key] = redactPayload(nested)
		}
		return out
	case []any:
		out := make([]any, 0, len(item))
		for _, nested := range item {
			out = append(out, redactPayload(nested))
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(item))
		for _, nested := range item {
			out = append(out, redactPayload(nested))
		}
		return out
	case []map[string]string:
		out := make([]any, 0, len(item))
		for _, nested := range item {
			out = append(out, redactPayload(nested))
		}
		return out
	case []string:
		out := make([]any, 0, len(item))
		for _, nested := range item {
			out = append(out, redactPayload(nested))
		}
		return out
	case string:
		return RedactString(item)
	default:
		return item
	}
}

// CompactOptions управляет обрезкой payload для hooks.
type CompactOptions struct {
	StringLimit int
	ListLimit   int
	DepthLimit  int
}

// CompactPayload рекурсивно редактирует и обрезает payload для передачи hooks.
func CompactPayload(value any, options CompactOptions) any {
	if options.StringLimit <= 0 {
		options.StringLimit = 2000
	}
	if options.ListLimit <= 0 {
		options.ListLimit = 20
	}
	if options.DepthLimit <= 0 {
		options.DepthLimit = 5
	}
	return compactPayload(value, options.StringLimit, options.ListLimit, options.DepthLimit)
}

func compactPayload(value any, stringLimit int, listLimit int, depth int) any {
	if depth <= 0 {
		return "<max depth>"
	}
	switch item := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, nested := range item {
			if IsSensitiveKey(key) {
				out[key] = RedactedValue
				continue
			}
			out[key] = compactPayload(nested, stringLimit, listLimit, depth-1)
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for key, nested := range item {
			if IsSensitiveKey(key) {
				out[key] = RedactedValue
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
		return clipped(RedactString(item), stringLimit)
	case int, int64, float64, bool, nil:
		return item
	default:
		return clipped(RedactString(fmt.Sprint(item)), stringLimit)
	}
}

// IsSensitiveKey сообщает, нужно ли целиком скрыть значение поля.
func IsSensitiveKey(key string) bool {
	lowered := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	compact := strings.ReplaceAll(lowered, "_", "")
	parts := strings.FieldsFunc(lowered, func(ch rune) bool { return ch == '_' })
	if strings.HasSuffix(lowered, "api_key") || strings.HasSuffix(lowered, "_token") {
		return true
	}
	if strings.HasSuffix(compact, "apikey") || strings.HasSuffix(compact, "token") || strings.HasSuffix(compact, "secret") || strings.HasSuffix(compact, "secretkey") {
		return true
	}
	for _, part := range parts {
		if sensitiveKeys[part] {
			return true
		}
	}
	return false
}

// RedactString сохраняет контекст строки, но заменяет похожие на секреты значения.
func RedactString(text string) string {
	text = redactAuthorizationValues(text)
	text = redactBearerValues(text)
	return redactAssignedValues(text)
}

func redactAuthorizationValues(text string) string {
	return authorizationValuePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := authorizationValuePattern.FindStringSubmatch(match)
		if len(parts) != 3 || !looksLikeBearerSecret(parts[2]) {
			return match
		}
		return parts[1] + RedactedValue
	})
}

func redactBearerValues(text string) string {
	return bearerValuePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := bearerValuePattern.FindStringSubmatch(match)
		if len(parts) != 3 || !looksLikeBearerSecret(parts[2]) {
			return match
		}
		return parts[1] + RedactedValue
	})
}

func redactAssignedValues(text string) string {
	return assignmentValuePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := assignmentValuePattern.FindStringSubmatch(match)
		if len(parts) != 6 || !shouldRedactAssignedValue(parts[1], parts[4]) {
			return match
		}
		return parts[1] + parts[2] + parts[3] + RedactedValue + parts[5]
	})
}

func shouldRedactAssignedValue(key string, value string) bool {
	if isPlaceholderSecret(value) {
		return false
	}
	keyKind := sensitiveAssignmentKind(key)
	if keyKind == "password" || keyKind == "secret" {
		return value != ""
	}
	if keyKind == "token" {
		return looksLikeTokenSecret(value)
	}
	return false
}

func sensitiveAssignmentKind(key string) string {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	compact := strings.ReplaceAll(normalized, "_", "")
	parts := strings.FieldsFunc(normalized, func(ch rune) bool { return ch == '_' })
	for _, part := range parts {
		switch part {
		case "password":
			return "password"
		case "secret":
			return "secret"
		case "apikey", "authorization", "token":
			return "token"
		}
	}
	if strings.HasPrefix(compact, "password") || strings.HasSuffix(compact, "password") {
		return "password"
	}
	if strings.HasSuffix(compact, "secret") || strings.HasSuffix(compact, "secretkey") {
		return "secret"
	}
	if strings.HasSuffix(compact, "apikey") || strings.HasSuffix(compact, "token") {
		return "token"
	}
	return ""
}

func looksLikeBearerSecret(value string) bool {
	if isPlaceholderSecret(value) {
		return false
	}
	if len(value) >= 16 {
		return true
	}
	return len(value) >= 8 && (hasKnownSecretPrefix(value) || strings.ContainsAny(value, "._-=/+") || hasDigit(value))
}

func looksLikeTokenSecret(value string) bool {
	if isPlaceholderSecret(value) {
		return false
	}
	if hasKnownSecretPrefix(value) {
		return true
	}
	if len(value) >= 20 {
		return true
	}
	return len(value) >= 8 && (strings.ContainsAny(value, "._-=/+") || hasDigit(value))
}

func hasKnownSecretPrefix(value string) bool {
	lowered := strings.ToLower(value)
	prefixes := []string{"sk-", "ghp_", "gho_", "github_pat_", "xoxb-", "xoxp-", "ya29.", "akia"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	return false
}

func hasDigit(value string) bool {
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			return true
		}
	}
	return false
}

func isPlaceholderSecret(value string) bool {
	lowered := strings.ToLower(strings.TrimSpace(value))
	if lowered == "" || lowered == RedactedValue {
		return true
	}
	return placeholderSecrets[lowered]
}

func clipped(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + fmt.Sprintf("\n...[clipped %d chars]", len(text)-limit)
}
