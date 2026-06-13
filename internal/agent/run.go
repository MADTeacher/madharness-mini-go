package agent

import (
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Run запускает агентский цикл до финального ответа или max_turns.
func Run(task string, cfg *config.Config) (string, string, error) {
	return runWithClient(task, cfg, model.New(cfg))
}

func runWithClient(task string, cfg *config.Config, client chatClient) (string, string, error) {
	tr, err := trace.New(cfg, "run")
	if err != nil {
		return "", "", err
	}
	registry, err := tools.NewRegistry(cfg, builtin.Provider{})
	if err != nil {
		return "", tr.Path, err
	}
	context, err := BaseContext(cfg, task)
	if err != nil {
		return "", tr.Path, err
	}
	result, err := runModelLoop(client, tr, context, registry, cfg.Data.MaxTurns)
	return result, tr.Path, err
}
