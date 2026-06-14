package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func cleanEnv(t *testing.T) {
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_SUBAGENTS", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_ENABLED", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_MODE", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	t.Setenv("MADHARNESS_MINI_APPROVAL_MODE", "")
	t.Setenv("MADHARNESS_MINI_YOLO", "")
}

func TestDefaultsMergeWithFile(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{"workspace_root": ".", "allow_shell": true})

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base_url = %q", cfg.Data.BaseURL)
	}
	if !cfg.Data.AllowShell {
		t.Fatal("allow_shell should remain true")
	}
	if cfg.Data.ContextMaxTokens != 60000 || cfg.Data.ContextKeepRecentTurns != 3 {
		t.Fatalf("context defaults = %d, %d", cfg.Data.ContextMaxTokens, cfg.Data.ContextKeepRecentTurns)
	}
}

func TestConfigIgnoresLegacyProviderFields(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{
		"provider":  "kodikrouter",
		"providers": map[string]any{"kodikrouter": map[string]any{"base_url": "https://example.test/v1"}},
	})

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base_url = %q", cfg.Data.BaseURL)
	}
}

func TestEnvFileOverridesConfig(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{"model": "old", "base_url": "https://old.example/v1"})
	env := "MADHARNESS_MINI_BASE_URL=https://new.example/v1\nMADHARNESS_MINI_MODEL=deepseek/deepseek-v4-flash\nMADHARNESS_MINI_API_KEY=secret\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.BaseURL != "https://new.example/v1" || cfg.Data.APIKey != "secret" {
		t.Fatalf("env was not applied: %+v", cfg.Data)
	}
}

func TestEnvFileOverridesImageSettingsWithTypes(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{"workspace_root": ".", "allow_shell": true})
	env := "MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT=true\nMADHARNESS_MINI_MAX_IMAGE_BYTES=42\nMADHARNESS_MINI_IMAGE_DETAIL=high\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Data.SupportsImageInput || cfg.Data.MaxImageBytes != 42 || cfg.Data.ImageDetail != "high" {
		t.Fatalf("image settings were not applied: %+v", cfg.Data)
	}
}

func TestEnvFileOverridesOrchestrationSettings(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{"workspace_root": ".", "allow_shell": true})
	env := "MADHARNESS_MINI_ORCHESTRATION_ENABLED=false\nMADHARNESS_MINI_ORCHESTRATION_MODE=requested\nMADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS=3\nMADHARNESS_MINI_MAX_PARALLEL_SUBAGENTS=2\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.OrchestrationEnabled || cfg.Data.OrchestrationMode != "requested" {
		t.Fatalf("orchestration settings were not applied: %+v", cfg.Data)
	}
	if cfg.Data.MaxParallelToolCalls != 3 {
		t.Fatalf("max_parallel_tool_calls = %d", cfg.Data.MaxParallelToolCalls)
	}
	if cfg.Data.MaxParallelSubagents != 2 {
		t.Fatalf("max_parallel_subagents = %d", cfg.Data.MaxParallelSubagents)
	}
}

func TestEnvFileOverridesApprovalSettings(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{"workspace_root": ".", "allow_shell": true})
	env := "MADHARNESS_MINI_APPROVAL_MODE=ask\nMADHARNESS_MINI_YOLO=true\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.ApprovalMode != "ask" || !cfg.Data.YoloMode {
		t.Fatalf("approval settings were not applied: %+v", cfg.Data)
	}
}

func TestMaxParallelToolCallsNormalizesToOne(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	mustWriteConfig(t, root, map[string]any{"max_parallel_tool_calls": 0, "max_parallel_subagents": 0})

	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.MaxParallelToolCalls != 1 {
		t.Fatalf("max_parallel_tool_calls = %d", cfg.Data.MaxParallelToolCalls)
	}
	if cfg.Data.MaxParallelSubagents != 1 {
		t.Fatalf("max_parallel_subagents = %d", cfg.Data.MaxParallelSubagents)
	}
}

func TestConfigCloneCopiesMutableSettings(t *testing.T) {
	cleanEnv(t)
	cfg, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Data.ProtectedPaths = []string{"secret.txt"}
	cfg.Data.Headers = map[string]string{"X-Test": "one"}

	clone := cfg.Clone()
	clone.Data.ProtectedPaths[0] = "changed.txt"
	clone.Data.Headers["X-Test"] = "two"
	clone.Data.Headers["X-New"] = "three"

	if cfg.Data.ProtectedPaths[0] != "secret.txt" {
		t.Fatalf("protected_paths share backing array: %v", cfg.Data.ProtectedPaths)
	}
	if cfg.Data.Headers["X-Test"] != "one" || cfg.Data.Headers["X-New"] != "" {
		t.Fatalf("headers share backing map: %v", cfg.Data.Headers)
	}
}

func TestEnvFileRejectsInvalidImageSettings(t *testing.T) {
	cases := map[string]string{
		"bool":          "MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT=maybe\n",
		"int":           "MADHARNESS_MINI_MAX_IMAGE_BYTES=-1\n",
		"detail":        "MADHARNESS_MINI_IMAGE_DETAIL=microscope\n",
		"orchestration": "MADHARNESS_MINI_ORCHESTRATION_MODE=surprise\n",
		"approval":      "MADHARNESS_MINI_APPROVAL_MODE=surprise\n",
		"subagents":     "MADHARNESS_MINI_MAX_PARALLEL_SUBAGENTS=-1\n",
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			cleanEnv(t)
			root := t.TempDir()
			mustWriteConfig(t, root, map[string]any{"workspace_root": ".", "allow_shell": true})
			if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := New(root); err == nil {
				t.Fatal("expected invalid image setting error")
			}
		})
	}
}

func TestInitializeCreatesConfigWithAPIKey(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	path, changes, err := cfg.Initialize(InitOptions{
		BaseURL: "https://kodikrouter.ru/api/v1",
		Model:   "deepseek/deepseek-v4-flash",
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) == 0 || filepath.Base(path) != "config.json" {
		t.Fatalf("unexpected init result path=%s changes=%v", path, changes)
	}
	data := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["api_key"] != "secret" {
		t.Fatalf("api_key = %v", data["api_key"])
	}
}

func TestInitializePersistsFlagsButNotEnvOverrides(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	env := "MADHARNESS_MINI_MODEL=env-model\nMADHARNESS_MINI_BASE_URL=https://env.example/v1\nMADHARNESS_MINI_API_KEY=env-secret\nMADHARNESS_MINI_APPROVAL_MODE=ask\nMADHARNESS_MINI_YOLO=true\nMADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS=4\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Data.YoloMode || cfg.Data.ApprovalMode != "ask" || cfg.Data.Model != "env-model" {
		t.Fatalf("env was not applied before init: %+v", cfg.Data)
	}
	path, _, err := cfg.Initialize(InitOptions{
		Model:  "flag-model",
		APIKey: "flag-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	data := readConfigMap(t, path)
	if data["model"] != "flag-model" || data["api_key"] != "flag-secret" {
		t.Fatalf("explicit init flags were not persisted: %v", data)
	}
	if data["base_url"] != "https://openrouter.ai/api/v1" {
		t.Fatalf("base_url persisted env override: %v", data["base_url"])
	}
	if data["approval_mode"] != "deny" || data["yolo_mode"] != false || data["max_parallel_tool_calls"] != float64(1) {
		t.Fatalf("env-only settings leaked into config: %v", data)
	}
}

func TestEnsureDirsDoesNotPersistEnvOverrides(t *testing.T) {
	cleanEnv(t)
	root := t.TempDir()
	env := "MADHARNESS_MINI_APPROVAL_MODE=ask\nMADHARNESS_MINI_YOLO=true\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Data.YoloMode || cfg.Data.ApprovalMode != "ask" {
		t.Fatalf("env was not applied before EnsureDirs: %+v", cfg.Data)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	data := readConfigMap(t, filepath.Join(root, StateDir, "config.json"))
	if data["approval_mode"] != "deny" || data["yolo_mode"] != false {
		t.Fatalf("EnsureDirs persisted env-only settings: %v", data)
	}
}

func mustWriteConfig(t *testing.T, root string, data map[string]any) {
	t.Helper()
	dir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readConfigMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}
