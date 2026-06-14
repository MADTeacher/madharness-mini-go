package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

// CommandConfig описывает один hook из `.madharness-mini/hooks.json`.
type CommandConfig struct {
	ID             string
	Mode           string
	Event          string
	Match          map[string]any
	Command        string
	Args           []string
	CWD            string
	Env            map[string]string
	TimeoutSeconds float64
}

// LoadCommandConfigs читает hooks.json; отсутствие файла означает пустой набор.
func LoadCommandConfigs(cfg *config.Config) ([]CommandConfig, error) {
	path := filepath.Join(cfg.StateDir, "hooks.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("invalid hooks config JSON: %s: %w", path, err)
	}
	hooks, ok := root["hooks"].([]any)
	if !ok {
		if _, exists := root["hooks"]; !exists {
			hooks = nil
		} else {
			return nil, fmt.Errorf("invalid hooks config: hooks must be list")
		}
	}
	pol := policy.New(cfg)
	configs := []CommandConfig{}
	for index, rawHook := range hooks {
		item, ok := rawHook.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid hook #%d: expected object", index+1)
		}
		if enabled, exists := item["enabled"]; exists && enabled != true {
			continue
		}
		parsed, err := parseCommandConfig(index+1, item, pol)
		if err != nil {
			return nil, err
		}
		configs = append(configs, parsed)
	}
	return configs, nil
}

func parseCommandConfig(index int, item map[string]any, pol *policy.Policy) (CommandConfig, error) {
	id, ok := item["id"].(string)
	if !ok || !safeHookID(id) {
		return CommandConfig{}, fmt.Errorf("invalid hook #%d: id must be non-empty safe string", index)
	}
	mode, err := hookMode(item)
	if err != nil {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: mode must be enforce or observe", id)
	}
	event, ok := item["event"].(string)
	if !ok || !Events[event] {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: event must be one of: %s", id, eventNames())
	}
	command, ok := item["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: command must be non-empty string", id)
	}
	args, err := stringList(item, "args", nil)
	if err != nil {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: args must be list of strings", id)
	}
	cwdRaw, ok := item["cwd"].(string)
	if !ok {
		if _, exists := item["cwd"]; exists {
			return CommandConfig{}, fmt.Errorf("invalid hook %s: cwd must be string", id)
		}
		cwdRaw = "."
	}
	cwd, err := pol.SafePath(cwdRaw)
	if err != nil {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: %w", id, err)
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: cwd is not a directory", id)
	}
	env, err := stringMap(item, "env")
	if err != nil {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: env must be object of strings", id)
	}
	match, err := matchMap(item)
	if err != nil {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: match must be object with scalar values", id)
	}
	timeout, err := timeoutSeconds(item)
	if err != nil {
		return CommandConfig{}, fmt.Errorf("invalid hook %s: timeout_seconds must be positive number", id)
	}
	return CommandConfig{
		ID:             id,
		Mode:           mode,
		Event:          event,
		Match:          match,
		Command:        command,
		Args:           args,
		CWD:            cwd,
		Env:            env,
		TimeoutSeconds: timeout,
	}, nil
}

func hookMode(item map[string]any) (string, error) {
	raw, exists := item["mode"]
	if !exists {
		return ModeEnforce, nil
	}
	mode, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("invalid mode")
	}
	switch strings.TrimSpace(mode) {
	case "", ModeEnforce:
		return ModeEnforce, nil
	case ModeObserve:
		return ModeObserve, nil
	default:
		return "", fmt.Errorf("invalid mode")
	}
}

func safeHookID(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, ch := range value {
		if ch > 127 {
			return false
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		if ch == '_' || ch == '-' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func stringList(item map[string]any, key string, fallback []string) ([]string, error) {
	raw, exists := item[key]
	if !exists {
		return fallback, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("not a list")
	}
	out := []string{}
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("not a string")
		}
		out = append(out, text)
	}
	return out, nil
}

func stringMap(item map[string]any, key string) (map[string]string, error) {
	raw, exists := item[key]
	if !exists {
		return map[string]string{}, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object")
	}
	out := map[string]string{}
	for key, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("not a string")
		}
		out[key] = text
	}
	return out, nil
}

func matchMap(item map[string]any) (map[string]any, error) {
	raw, exists := item["match"]
	if !exists {
		return map[string]any{}, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object")
	}
	out := map[string]any{}
	for key, value := range values {
		if !validMatchValue(value) {
			return nil, fmt.Errorf("invalid value")
		}
		out[key] = value
	}
	return out, nil
}

func validMatchValue(value any) bool {
	if isScalar(value) {
		return true
	}
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if !isScalar(item) {
			return false
		}
	}
	return true
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, float64, bool:
		return true
	default:
		return false
	}
}

func timeoutSeconds(item map[string]any) (float64, error) {
	raw, exists := item["timeout_seconds"]
	if !exists {
		return 5, nil
	}
	value, ok := raw.(float64)
	if !ok || value <= 0 {
		return 0, fmt.Errorf("invalid timeout")
	}
	return value, nil
}

func eventNames() string {
	names := []string{
		"approval_decision",
		"approval_request",
		"after_model_call",
		"after_tool_call",
		"before_model_call",
		"before_tool_call",
		"session_end",
		"session_error",
		"session_start",
	}
	return strings.Join(names, ", ")
}
