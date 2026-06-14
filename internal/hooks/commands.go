package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

var safeInheritedEnv = map[string]bool{
	"ComSpec":    true,
	"HOME":       true,
	"LANG":       true,
	"LC_ALL":     true,
	"PATH":       true,
	"SystemRoot": true,
	"TEMP":       true,
	"TMP":        true,
	"TMPDIR":     true,
	"USER":       true,
	"WINDIR":     true,
}

// CommandProvider адаптирует запись hooks.json к общему Provider.
type CommandProvider struct {
	Config CommandConfig
}

// ID возвращает стабильное имя hook для trace.
func (p CommandProvider) ID() string {
	return p.Config.ID
}

// Matches проверяет имя события и простой exact-match по payload.
func (p CommandProvider) Matches(event Event) bool {
	if event.Name != p.Config.Event {
		return false
	}
	for key, expected := range p.Config.Match {
		var actual any
		if key == "kind" {
			actual = event.Kind
		} else {
			actual = event.HookData[key]
		}
		if !matchValue(actual, expected) {
			return false
		}
	}
	return true
}

// Handle передаёт событие в stdin и читает JSON-решение из stdout.
func (p CommandProvider) Handle(event Event) (Decision, error) {
	payload, err := json.Marshal(hookPayload(event))
	if err != nil {
		return Allow(), err
	}
	timeout := time.Duration(p.Config.TimeoutSeconds * float64(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Config.Command, p.Config.Args...)
	cmd.Dir = p.Config.CWD
	cmd.Env = hookEnv(p.Config.Env)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Allow(), fmt.Errorf("hook timed out after %ss", trimFloat(p.Config.TimeoutSeconds))
	}
	if err != nil {
		detail := fmt.Sprintf("hook exited with error: %v", err)
		if text := strings.TrimSpace(stderr.String()); text != "" {
			detail += ": " + tools.Clipped(text, 1000)
		}
		return Allow(), fmt.Errorf("%s", detail)
	}
	return decisionFromStdout(stdout.String())
}

func hookPayload(event Event) map[string]any {
	return map[string]any{
		"version":  SchemaVersion,
		"event":    event.Name,
		"kind":     event.Kind,
		"trace_id": event.TraceID,
		"data":     event.HookData,
	}
}

func decisionFromStdout(stdout string) (Decision, error) {
	text := strings.TrimSpace(stdout)
	if text == "" {
		return Allow(), nil
	}
	raw := map[string]any{}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return Allow(), fmt.Errorf("invalid hook stdout JSON: %w", err)
	}
	okValue := true
	if rawOK, exists := raw["ok"]; exists {
		parsed, ok := rawOK.(bool)
		if !ok {
			return Allow(), fmt.Errorf("invalid hook stdout JSON: ok must be boolean")
		}
		okValue = parsed
	}
	block := strings.TrimSpace(fmt.Sprint(raw["block"]))
	if raw["block"] == nil {
		block = ""
	}
	message := strings.TrimSpace(fmt.Sprint(raw["message"]))
	if raw["message"] == nil {
		message = ""
	}
	if !okValue || block != "" {
		if block == "" {
			block = message
		}
		if block == "" {
			block = "blocked by hook"
		}
		return Decision{OK: false, Block: block, Message: message, Source: "hook"}, nil
	}
	return Decision{OK: true, Message: message}, nil
}

func hookEnv(explicit map[string]string) []string {
	out := []string{}
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if !ok || !safeInheritedEnv[key] || strings.HasPrefix(key, "MADHARNESS_MINI_") {
			continue
		}
		out = append(out, item)
	}
	for key, value := range explicit {
		out = append(out, key+"="+value)
	}
	return out
}

func matchValue(actual any, expected any) bool {
	if options, ok := expected.([]any); ok {
		for _, option := range options {
			if scalarEqual(actual, option) {
				return true
			}
		}
		return false
	}
	return scalarEqual(actual, expected)
}

func scalarEqual(actual any, expected any) bool {
	switch expectedValue := expected.(type) {
	case float64:
		actualNumber, ok := numberAsFloat(actual)
		return ok && actualNumber == expectedValue
	default:
		return actual == expected
	}
}

func numberAsFloat(value any) (float64, bool) {
	switch item := value.(type) {
	case int:
		return float64(item), true
	case int64:
		return float64(item), true
	case float64:
		return item, true
	default:
		return 0, false
	}
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
