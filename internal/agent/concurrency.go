package agent

import "github.com/MADTeacher/madharness-mini-go/internal/config"

func resolvedMaxParallelToolCalls(cfg *config.Config, options RunOptions) int {
	if options.MaxParallelToolCalls > 0 {
		return options.MaxParallelToolCalls
	}
	return cfg.Data.MaxParallelToolCalls
}
