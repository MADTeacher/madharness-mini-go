package agentcontext

import (
	"path/filepath"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/instructions"
	"github.com/MADTeacher/madharness-mini-go/internal/prompt"
)

// BaseContext готовит стартовый контекст ask/run: system prompt, AGENTS.md и задачу.
func BaseContext(cfg *config.Config, task string, providers ...Provider) (*Manager, error) {
	system, err := prompt.Load("system")
	if err != nil {
		return nil, err
	}
	projectInstructions, err := instructions.LoadProject(cfg)
	if err != nil {
		return nil, err
	}
	context := NewManager(task, Options{
		MaxTokens:       cfg.Data.ContextMaxTokens,
		KeepRecentTurns: cfg.Data.ContextKeepRecentTurns,
		Providers:       providers,
	})
	context.AddFragment(Fragment{
		ID:        "system",
		Source:    "internal/prompt/prompts/system.md",
		Text:      system,
		Priority:  0,
		Placement: "system",
	})
	if projectInstructions != "" {
		context.AddFragment(Fragment{
			ID:        "project-instructions",
			Source:    filepath.Join(cfg.Root, instructions.ProjectDocFilename),
			Text:      "# Project instructions\n\n" + projectInstructions,
			Priority:  10,
			Placement: "system",
		})
	}
	return context, nil
}
