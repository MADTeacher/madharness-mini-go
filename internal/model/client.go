package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

// RateLimitError выделяет HTTP 429, чтобы agent loop мог сделать один retry.
type RateLimitError struct {
	Status            int
	Body              string
	RetryAfter        string
	RetryAfterSeconds int
	HasRetryAfter     bool
}

func (e *RateLimitError) Error() string {
	return "достигнут лимит LLM API (HTTP 429); попробуйте позже, смените модель/ключ или проверьте лимиты провайдера"
}

// Client отправляет POST /chat/completions.
type Client struct {
	cfg  *config.Config
	http *http.Client
}

// New создаёт model client с таймаутом, достаточным для длинного ответа модели.
func New(cfg *config.Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 120 * time.Second}}
}

// Settings возвращает эффективные параметры model call.
func (c *Client) Settings() map[string]any {
	headers := map[string]string{}
	for key, value := range c.cfg.Data.Headers {
		headers[key] = value
	}
	return map[string]any{
		"base_url": c.cfg.Data.BaseURL,
		"api_key":  c.cfg.Data.APIKey,
		"headers":  headers,
		"model":    c.cfg.Data.Model,
	}
}

// Chat вызывает OpenAI-compatible /chat/completions и возвращает сырой JSON.
func (c *Client) Chat(messages []map[string]any, tools []map[string]any) (map[string]any, error) {
	if c.cfg.Data.APIKey == "" {
		return nil, fmt.Errorf("нет ключа API для LLM API: запустите madharness-mini init --api-key ... или задайте MADHARNESS_MINI_API_KEY")
	}
	payload := map[string]any{
		"model":       c.cfg.Data.Model,
		"messages":    messages,
		"temperature": c.cfg.Data.Temperature,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
		payload["parallel_tool_calls"] = c.cfg.Data.MaxParallelToolCalls > 1
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(c.cfg.Data.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.Data.APIKey)
	for key, value := range c.cfg.Data.Headers {
		req.Header.Set(key, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		err := &RateLimitError{Status: resp.StatusCode, Body: string(raw), RetryAfter: resp.Header.Get("Retry-After")}
		if seconds, ok := ParseRetryAfter(err.RetryAfter); ok {
			err.RetryAfterSeconds = seconds
			err.HasRetryAfter = true
		}
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM API HTTP %d: %s", resp.StatusCode, string(raw))
	}
	result := map[string]any{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}
