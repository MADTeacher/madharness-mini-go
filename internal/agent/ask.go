package agent

import (
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
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
	eventBus := events.NewBus(events.NewTraceSubscriber(tr), hookManager)
	defer eventBus.Close()
	publishEvent(eventBus, events.Event{Name: "session_start", Kind: "ask", HookData: map[string]any{
		"task_preview": truncateForHook(task, 1000),
		"cwd":          cfg.CWD,
	}})
	context, err := BaseContext(cfg, task)
	if err != nil {
		publishSessionEndTrace(eventBus, "error: "+err.Error())
		emitSessionError(eventBus, "ask", err, nil)
		return "", tr.Path, err
	}
	messages, err := context.Messages(nil)
	if err != nil {
		_ = tr.Write("context_error", map[string]any{
			"error":          err.Error(),
			"context_report": safeContextReport(context),
		})
		publishSessionEndTrace(eventBus, "error: "+err.Error())
		emitSessionError(eventBus, "ask", err, nil)
		return "", tr.Path, err
	}
	contextReport := context.Report()
	publishEvent(eventBus, events.Event{
		Name: "before_model_call",
		Kind: "ask",
		HookData: map[string]any{
			"tools_count":    0,
			"context_report": contextReport,
		},
		TraceName: "model_call_started",
		TraceData: map[string]any{
			"tools_count":    0,
			"context_report": contextReport,
		},
	})
	raw, err := callModelWithRateLimitRetry(client, tr, messages, nil, nil)
	if err != nil {
		_ = tr.Write("model_error", map[string]any{"error": err.Error()})
		publishSessionEndTrace(eventBus, "error: "+err.Error())
		emitSessionError(eventBus, "ask", err, nil)
		return "", tr.Path, err
	}
	message, err := responseMessage(raw)
	if err != nil {
		_ = tr.Write("model_error", map[string]any{"error": err.Error()})
		publishSessionEndTrace(eventBus, "error: "+err.Error())
		emitSessionError(eventBus, "ask", err, nil)
		return "", tr.Path, err
	}
	publishEvent(eventBus, events.Event{
		Name:      "after_model_call",
		Kind:      "ask",
		HookData:  map[string]any{"message": modelMessageSummary(message)},
		TraceName: "model_call_finished",
		TraceData: map[string]any{"raw": raw},
	})
	content := messageContent(message)
	publishSessionEnd(eventBus, "ask", content, map[string]any{
		"status":         "done",
		"turns":          1,
		"result_preview": truncateForHook(content, 1000),
	})
	return content, tr.Path, nil
}
