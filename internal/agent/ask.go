package agent

import (
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/hooks"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Ask отправляет один запрос к модели без инструментов.
func Ask(task string, cfg *config.Config) (string, string, error) {
	return askWithClient(task, cfg, model.New(cfg))
}

func askWithClient(task string, cfg *config.Config, client chatClient) (string, string, error) {
	tr, err := trace.New(cfg, "ask")
	if err != nil {
		return "", "", err
	}
	hookManager, err := hooks.FromConfig(cfg, tr)
	if err != nil {
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		return "", tr.Path, err
	}
	hookManager.Emit("session_start", "ask", map[string]any{
		"task_preview": truncateForHook(task, 1000),
		"cwd":          cfg.CWD,
	})
	context, err := BaseContext(cfg, task)
	if err != nil {
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "ask", err, nil)
		return "", tr.Path, err
	}
	messages, err := context.Messages(nil)
	if err != nil {
		_ = tr.Write("context_error", map[string]any{
			"error":          err.Error(),
			"context_report": safeContextReport(context),
		})
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "ask", err, nil)
		return "", tr.Path, err
	}
	contextReport := context.Report()
	_ = tr.Write("model_call_started", map[string]any{
		"tools_count":    0,
		"context_report": contextReport,
	})
	hookManager.Emit("before_model_call", "ask", map[string]any{
		"tools_count":    0,
		"context_report": contextReport,
	})
	raw, err := callModelWithRateLimitRetry(client, tr, messages, nil, nil)
	if err != nil {
		_ = tr.Write("model_error", map[string]any{"error": err.Error()})
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "ask", err, nil)
		return "", tr.Path, err
	}
	_ = tr.Write("model_call_finished", map[string]any{"raw": raw})
	message, err := responseMessage(raw)
	if err != nil {
		_ = tr.Write("model_error", map[string]any{"error": err.Error()})
		_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
		emitSessionError(hookManager, "ask", err, nil)
		return "", tr.Path, err
	}
	hookManager.Emit("after_model_call", "ask", map[string]any{
		"message": modelMessageSummary(message),
	})
	content := messageContent(message)
	_ = tr.Write("session_end", map[string]any{"result": content})
	hookManager.Emit("session_end", "ask", map[string]any{
		"status":         "done",
		"turns":          1,
		"result_preview": truncateForHook(content, 1000),
	})
	return content, tr.Path, nil
}
