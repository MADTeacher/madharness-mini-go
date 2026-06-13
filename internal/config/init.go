package config

import (
	"os"
	"path/filepath"
)

// InitOptions описывает явные значения, переданные командой init.
type InitOptions struct {
	Model   string
	BaseURL string
	APIKey  string
}

// Initialize записывает config.json и возвращает путь и реально изменённые поля.
func (c *Config) Initialize(opts InitOptions) (string, []string, error) {
	if err := os.MkdirAll(filepath.Join(c.StateDir, "traces"), 0o755); err != nil {
		return "", nil, err
	}
	path := filepath.Join(c.StateDir, "config.json")
	changes := []string{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		changes = append(changes, "created")
	} else if err != nil {
		return "", nil, err
	}
	data := c.Data
	if opts.Model != "" && data.Model != opts.Model {
		data.Model = opts.Model
		changes = append(changes, "model")
	}
	if opts.BaseURL != "" && data.BaseURL != opts.BaseURL {
		data.BaseURL = opts.BaseURL
		changes = append(changes, "base_url")
	}
	if opts.APIKey != "" && data.APIKey != opts.APIKey {
		data.APIKey = opts.APIKey
		changes = append(changes, "api_key")
	}
	if err := c.writeConfig(path, data); err != nil {
		return "", nil, err
	}
	c.Data = data
	c.refreshRoot()
	return path, changes, nil
}
