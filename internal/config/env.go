package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func (c *Config) applyEnv() error {
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
	if value := env["MADHARNESS_MINI_ORCHESTRATION_ENABLED"]; value != "" {
		parsed, err := parseBoolEnv("MADHARNESS_MINI_ORCHESTRATION_ENABLED", value)
		if err != nil {
			return err
		}
		c.Data.OrchestrationEnabled = parsed
	}
	if value := env["MADHARNESS_MINI_ORCHESTRATION_MODE"]; value != "" {
		mode := strings.ToLower(strings.TrimSpace(value))
		if !OrchestrationModeValues[mode] {
			return fmt.Errorf("invalid MADHARNESS_MINI_ORCHESTRATION_MODE: %s; allowed: auto, off, requested, required", mode)
		}
		c.Data.OrchestrationMode = mode
	}
	if value := env["MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT"]; value != "" {
		parsed, err := parseBoolEnv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", value)
		if err != nil {
			return err
		}
		c.Data.SupportsImageInput = parsed
	}
	if value := env["MADHARNESS_MINI_MAX_IMAGE_BYTES"]; value != "" {
		parsed, err := parseIntEnv("MADHARNESS_MINI_MAX_IMAGE_BYTES", value)
		if err != nil {
			return err
		}
		c.Data.MaxImageBytes = parsed
	}
	if value := env["MADHARNESS_MINI_IMAGE_DETAIL"]; value != "" {
		detail := strings.TrimSpace(value)
		if !ImageDetailValues[detail] {
			return fmt.Errorf("invalid MADHARNESS_MINI_IMAGE_DETAIL: %s; allowed: auto, high, low, original", detail)
		}
		c.Data.ImageDetail = detail
	}
	return nil
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

func parseBoolEnv(name string, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s: %s; expected true or false", name, value)
	}
}

func parseIntEnv(name string, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s; expected integer", name, value)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("invalid %s: %s; expected non-negative integer", name, value)
	}
	return parsed, nil
}
