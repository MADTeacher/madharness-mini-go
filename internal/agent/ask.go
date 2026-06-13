package agent

import (
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Ask отправляет один запрос к модели без инструментов.
func Ask(task string, cfg *config.Config) (string, string, error) {
	tr, err := trace.New(cfg, "ask")
	if err != nil {
		return "", "", err
	}
	messages, err := BaseMessages(cfg, task)
	if err != nil {
		return "", "", err
	}
	_ = tr.Write("model_call_started", map[string]any{"tools_count": 0})
	raw, err := callModelWithRateLimitRetry(model.New(cfg), tr, messages, nil, nil)
	if err != nil {
		_ = tr.Write("model_error", map[string]any{"error": err.Error()})
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		return "", tr.Path, err
	}
	_ = tr.Write("model_call_finished", map[string]any{"raw": raw})
	message, err := responseMessage(raw)
	if err != nil {
		_ = tr.Write("model_error", map[string]any{"error": err.Error()})
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		return "", tr.Path, err
	}
	content := messageContent(message)
	_ = tr.Write("session_end", map[string]any{"result": content})
	return content, tr.Path, nil
}
