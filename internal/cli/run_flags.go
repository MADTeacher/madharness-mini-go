package cli

import "fmt"

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
