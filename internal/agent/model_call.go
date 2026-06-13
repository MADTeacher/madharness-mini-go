package agent

import (
	"errors"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

const rateLimitRetryMaxSeconds = 60

var sleep = time.Sleep

type chatClient interface {
	Chat(messages []map[string]any, tools []map[string]any) (map[string]any, error)
}

func callModelWithRateLimitRetry(
	client chatClient,
	tr *trace.Trace,
	messages []map[string]any,
	tools []map[string]any,
	traceData map[string]any,
) (map[string]any, error) {
	raw, err := client.Chat(messages, tools)
	if err == nil {
		return raw, nil
	}
	var limited *model.RateLimitError
	if !errors.As(err, &limited) {
		return nil, err
	}
	if !limited.HasRetryAfter ||
		limited.RetryAfterSeconds <= 0 ||
		limited.RetryAfterSeconds > rateLimitRetryMaxSeconds {
		return nil, err
	}
	fields := map[string]any{
		"status":              limited.Status,
		"retry_after":         limited.RetryAfter,
		"retry_after_seconds": limited.RetryAfterSeconds,
	}
	for key, value := range traceData {
		fields[key] = value
	}
	_ = tr.Write("model_rate_limit_retry", fields)
	sleep(time.Duration(limited.RetryAfterSeconds) * time.Second)
	return client.Chat(messages, tools)
}
