package cli

import (
	"flag"
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/agent"
)

type runFlagValues struct {
	fs                  *flag.FlagSet
	orchestration       *string
	noOrchestrate       *bool
	orchestrate         *bool
	orchestrateRequired *bool
	maxParallelTools    *int
}

func registerRunFlags(fs *flag.FlagSet) runFlagValues {
	return runFlagValues{
		fs:                  fs,
		orchestration:       fs.String("orchestration", "", "orchestration mode: off, requested, auto, required"),
		noOrchestrate:       fs.Bool("no-orchestrate", false, "do not show delegate_task to the parent agent"),
		orchestrate:         fs.Bool("orchestrate", false, "make delegate_task available to the parent agent"),
		orchestrateRequired: fs.Bool("orchestrate-required", false, "strict mode: parent coordinates work through subagents"),
		maxParallelTools:    fs.Int("max-parallel-tool-calls", 0, "per-run max parallel read-only tool calls; default uses config"),
	}
}

func (f runFlagValues) options() (agent.RunOptions, error) {
	mode, err := selectedOrchestrationMode(*f.orchestration, *f.noOrchestrate, *f.orchestrate, *f.orchestrateRequired)
	if err != nil {
		return agent.RunOptions{}, err
	}
	maxParallel, err := selectedMaxParallelToolCalls(f.fs, *f.maxParallelTools)
	if err != nil {
		return agent.RunOptions{}, err
	}
	return agent.RunOptions{
		OrchestrationMode:    mode,
		MaxParallelToolCalls: maxParallel,
	}, nil
}

func selectedOrchestrationMode(raw string, noOrchestrate bool, orchestrate bool, required bool) (string, error) {
	choices := []string{}
	if raw != "" {
		choices = append(choices, raw)
	}
	if noOrchestrate {
		choices = append(choices, "off")
	}
	if orchestrate {
		choices = append(choices, "auto")
	}
	if required {
		choices = append(choices, "required")
	}
	if len(choices) > 1 {
		return "", fmt.Errorf("use only one orchestration flag")
	}
	if len(choices) == 0 {
		return "", nil
	}
	return choices[0], nil
}

func selectedMaxParallelToolCalls(fs *flag.FlagSet, value int) (int, error) {
	if !flagWasPassed(fs, "max-parallel-tool-calls") {
		return 0, nil
	}
	if value < 1 {
		return 0, fmt.Errorf("max-parallel-tool-calls must be >= 1")
	}
	return value, nil
}

func flagWasPassed(fs *flag.FlagSet, name string) bool {
	passed := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			passed = true
		}
	})
	return passed
}
