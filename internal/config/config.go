package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Settings хранит поля .madharness-mini/config.json, которые видит CLI и loop.
type Settings struct {
	Model                    string            `json:"model"`
	BaseURL                  string            `json:"base_url"`
	APIKey                   string            `json:"api_key"`
	Temperature              float64           `json:"temperature"`
	MaxTurns                 int               `json:"max_turns"`
	MaxParallelToolCalls     int               `json:"max_parallel_tool_calls"`
	ContextMaxTokens         int               `json:"context_max_tokens"`
	ContextKeepRecentTurns   int               `json:"context_keep_recent_turns"`
	WorkspaceRoot            string            `json:"workspace_root"`
	ProtectedPaths           []string          `json:"protected_paths"`
	AllowShell               bool              `json:"allow_shell"`
	ApprovalMode             string            `json:"approval_mode"`
	YoloMode                 bool              `json:"yolo_mode"`
	OrchestrationEnabled     bool              `json:"orchestration_enabled"`
	OrchestrationMode        string            `json:"orchestration_mode"`
	SubagentMaxTurns         int               `json:"subagent_max_turns"`
	SubagentContextMaxTokens int               `json:"subagent_context_max_tokens"`
	SupportsImageInput       bool              `json:"supports_image_input"`
	MaxImageBytes            int               `json:"max_image_bytes"`
	ImageDetail              string            `json:"image_detail"`
	Headers                  map[string]string `json:"headers,omitempty"`
}

// Config связывает настройки запуска с абсолютными путями текущего workspace.
type Config struct {
	CWD      string
	StateDir string
	Root     string
	Data     Settings
}

// New строит конфиг слоями: defaults, config.json, .env, переменные окружения.
func New(cwd string) (*Config, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		CWD:      absCWD,
		StateDir: filepath.Join(absCWD, StateDir),
		Data:     DefaultSettings(),
	}
	if err := cfg.loadFile(); err != nil {
		return nil, err
	}
	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}
	cfg.normalize()
	cfg.refreshRoot()
	return cfg, nil
}

func (c *Config) normalize() {
	if c.Data.MaxParallelToolCalls < 1 {
		c.Data.MaxParallelToolCalls = 1
	}
}

func (c *Config) loadFile() error {
	path := filepath.Join(c.StateDir, "config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &c.Data); err != nil {
		return err
	}
	if c.Data.Headers == nil {
		c.Data.Headers = map[string]string{}
	}
	return nil
}

func (c *Config) refreshRoot() {
	root := c.Data.WorkspaceRoot
	if root == "" {
		root = "."
		c.Data.WorkspaceRoot = root
	}
	if filepath.IsAbs(root) {
		c.Root = filepath.Clean(root)
		return
	}
	c.Root = filepath.Clean(filepath.Join(c.CWD, root))
}

// EnsureDirs создаёт служебный каталог и config.json, если запуск первый.
func (c *Config) EnsureDirs() error {
	if err := os.MkdirAll(filepath.Join(c.StateDir, "traces"), 0o755); err != nil {
		return err
	}
	path := filepath.Join(c.StateDir, "config.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return c.writeConfig(path, c.Data)
	} else if err != nil {
		return err
	}
	return nil
}

func (c *Config) writeConfig(path string, data Settings) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o600)
}
