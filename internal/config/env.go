package config

import (
	"os"
	"strings"
)

func (c *Config) applyEnv() {
	env := readEnvFile(c.CWD + "/.env")
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok && value != "" && strings.HasPrefix(key, "MADHARNESS_MINI_") {
			env[key] = value
		}
	}
	if value := env["MADHARNESS_MINI_MODEL"]; value != "" {
		c.Data.Model = value
	}
	if value := env["MADHARNESS_MINI_BASE_URL"]; value != "" {
		c.Data.BaseURL = value
	}
	if value := env["MADHARNESS_MINI_API_KEY"]; value != "" {
		c.Data.APIKey = value
	}
}

func readEnvFile(path string) map[string]string {
	data := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return data
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			data[key] = value
		}
	}
	return data
}
