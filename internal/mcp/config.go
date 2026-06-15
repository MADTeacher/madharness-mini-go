// Package mcp подключает stdio MCP-серверы как обычные tools харнесса.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

const defaultTimeoutSeconds = 20

// ServerConfig хранит проверенные настройки одного включённого MCP-сервера.
type ServerConfig struct {
	Name    string
	Command string
	Args    []string
	CWD     string
	Env     map[string]string
	Timeout time.Duration
}

// LoadServerConfigs читает `.madharness-mini/mcp.json`; отсутствие файла означает MCP off.
func LoadServerConfigs(cfg *config.Config, pol *policy.Policy) ([]ServerConfig, error) {
	path := filepath.Join(cfg.StateDir, "mcp.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("invalid MCP config JSON: %s: %w", path, err)
	}
	servers, ok := document["servers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid MCP config: servers must be object")
	}
	configs := []ServerConfig{}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		rawItem := servers[name]
		if !safeServerName(name) {
			return nil, fmt.Errorf("invalid MCP server name: %s", name)
		}
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid MCP server config for %s: expected object", name)
		}
		if enabled, _ := item["enabled"].(bool); !enabled {
			continue
		}
		parsed, err := parseServerConfig(name, item, pol)
		if err != nil {
			return nil, err
		}
		configs = append(configs, parsed)
	}
	return configs, nil
}

func parseServerConfig(name string, item map[string]any, pol *policy.Policy) (ServerConfig, error) {
	command, ok := item["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: command must be non-empty string", name)
	}
	args, err := stringListField(item, "args")
	if err != nil {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: %w", name, err)
	}
	rawCWD, ok := item["cwd"].(string)
	if _, exists := item["cwd"]; exists && !ok {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: cwd must be string", name)
	}
	if !ok {
		rawCWD = "."
	}
	cwd, err := pol.SafePath(rawCWD)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: %w", name, err)
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: cwd is not a directory", name)
	}
	if !info.IsDir() {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: cwd is not a directory", name)
	}
	env, err := stringMapField(item, "env")
	if err != nil {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: %w", name, err)
	}
	timeout, err := timeoutField(item)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("invalid MCP server %s: %w", name, err)
	}
	return ServerConfig{
		Name:    name,
		Command: command,
		Args:    args,
		CWD:     cwd,
		Env:     env,
		Timeout: timeout,
	}, nil
}

func stringListField(item map[string]any, name string) ([]string, error) {
	raw, exists := item[name]
	if !exists {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be list of strings", name)
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be list of strings", name)
		}
		out = append(out, text)
	}
	return out, nil
}

func stringMapField(item map[string]any, name string) (map[string]string, error) {
	raw, exists := item[name]
	if !exists {
		return map[string]string{}, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be object of strings", name)
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be object of strings", name)
		}
		out[key] = text
	}
	return out, nil
}

func timeoutField(item map[string]any) (time.Duration, error) {
	raw, exists := item["timeout_seconds"]
	if !exists {
		return defaultTimeoutSeconds * time.Second, nil
	}
	seconds, ok := raw.(float64)
	if !ok || seconds <= 0 {
		return 0, fmt.Errorf("timeout_seconds must be positive number")
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func safeServerName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
