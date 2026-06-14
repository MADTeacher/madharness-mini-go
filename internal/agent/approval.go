package agent

import (
	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
)

func approvalManagerForRun(cfg *config.Config, options RunOptions, bus *events.Bus, kind string) *approval.Manager {
	mode := cfg.Data.ApprovalMode
	if options.ApprovalMode != "" {
		mode = options.ApprovalMode
	}
	yolo := cfg.Data.YoloMode || options.YoloMode
	return &approval.Manager{
		Mode:     mode,
		Yolo:     yolo,
		Kind:     kind,
		Events:   bus,
		Prompter: options.ApprovalPrompter,
	}
}
