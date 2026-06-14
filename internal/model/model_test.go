package model

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	seconds, ok := ParseRetryAfter("7")
	if !ok || seconds != 7 {
		t.Fatalf("ParseRetryAfter = %d, %v", seconds, ok)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	value := time.Now().UTC().Add(30 * time.Second).Format(http.TimeFormat)
	seconds, ok := ParseRetryAfter(value)
	if !ok || seconds < 1 || seconds > 30 {
		t.Fatalf("ParseRetryAfter date = %d, %v", seconds, ok)
	}
}

func TestClientSettingsAndMissingKey(t *testing.T) {
	cfg := testConfig(t)
	cfg.Data.BaseURL = "https://kodikrouter.ru/api/v1"
	settings := New(cfg).Settings()
	if settings["base_url"] != "https://kodikrouter.ru/api/v1" {
		t.Fatalf("settings = %+v", settings)
	}
	if _, err := New(cfg).Chat([]map[string]any{{"role": "user", "content": "hello"}}, nil); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestClientRaisesRateLimitErrorFor429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"limited"}`))
	}))
	defer server.Close()
	cfg := testConfig(t)
	cfg.Data.BaseURL = server.URL
	cfg.Data.APIKey = "token"

	_, err := New(cfg).Chat([]map[string]any{{"role": "user", "content": "hello"}}, nil)
	limited, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("error = %T %v", err, err)
	}
	if limited.Status != 429 || limited.RetryAfterSeconds != 3 || limited.Body != `{"error":"limited"}` {
		t.Fatalf("limited = %+v", limited)
	}
}

func TestClientSendsChatCompletionsPayload(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()
	cfg := testConfig(t)
	cfg.Data.BaseURL = server.URL
	cfg.Data.APIKey = "token"

	raw, err := New(cfg).Chat([]map[string]any{{"role": "user", "content": "hello"}}, []map[string]any{{"type": "function"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["choices"].([]any); !ok {
		t.Fatalf("raw = %+v", raw)
	}
	if payload["parallel_tool_calls"] != false {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestClientEnablesParallelToolCallsWhenConfigured(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()
	cfg := testConfig(t)
	cfg.Data.BaseURL = server.URL
	cfg.Data.APIKey = "token"
	cfg.Data.MaxParallelToolCalls = 2

	if _, err := New(cfg).Chat([]map[string]any{{"role": "user", "content": "hello"}}, []map[string]any{{"type": "function"}}); err != nil {
		t.Fatal(err)
	}
	if payload["parallel_tool_calls"] != true {
		t.Fatalf("payload = %+v", payload)
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
