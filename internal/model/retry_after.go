// Package model реализует OpenAI-compatible Chat Completions client.
package model

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter переводит Retry-After в секунды ожидания.
func ParseRetryAfter(value string) (int, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
		return seconds, true
	}
	retryAt, err := time.Parse(time.RFC1123, raw)
	if err != nil {
		return 0, false
	}
	seconds := math.Ceil(time.Until(retryAt).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return int(seconds), true
}
