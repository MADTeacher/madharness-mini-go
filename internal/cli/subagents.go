package cli

import (
	"fmt"
	"sort"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/subagents"
)

// SubagentsCommand печатает найденных markdown-субагентов и диагностику frontmatter.
func SubagentsCommand(cfg *config.Config, command string, args ...string) (string, error) {
	index := subagents.Discover(cfg)
	switch command {
	case "list":
		if len(index.Subagents) == 0 {
			return "Субагенты не найдены.", nil
		}
		lines := []string{"Доступные субагенты:"}
		for _, name := range index.Names() {
			subagent := index.Subagents[name]
			lines = append(lines, fmt.Sprintf("- %s: %s (%s; %s; %s)", subagent.Name, subagent.Description, subagent.Profile, subagent.Source, subagent.Location))
		}
		return join(lines, "\n"), nil
	case "show":
		if len(args) != 1 {
			return "", fmt.Errorf("usage: madharness-mini subagents show <name>")
		}
		subagent, ok := index.Subagents[args[0]]
		if !ok {
			return "", fmt.Errorf("subagent not found: %s", args[0])
		}
		return renderSubagent(subagent), nil
	case "validate":
		return renderSubagentsValidation(index), nil
	default:
		return "", fmt.Errorf("unknown subagents command: %s", command)
	}
}

func renderSubagent(subagent subagents.Subagent) string {
	lines := []string{
		"name: " + subagent.Name,
		"description: " + subagent.Description,
		"profile: " + subagent.Profile,
		"tools: " + join(subagent.Tools, ", "),
		"source: " + subagent.Source,
		"location: " + subagent.Location,
	}
	if subagent.MaxTurns > 0 {
		lines = append(lines, fmt.Sprintf("max_turns: %d", subagent.MaxTurns))
	}
	if subagent.ContextMaxTokens > 0 {
		lines = append(lines, fmt.Sprintf("context_max_tokens: %d", subagent.ContextMaxTokens))
	}
	if len(subagent.Metadata) > 0 {
		lines = append(lines, "metadata:")
		keys := make([]string, 0, len(subagent.Metadata))
		for key := range subagent.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("  %s: %s", key, subagent.Metadata[key]))
		}
	}
	lines = append(lines, "", "prompt:", subagent.Prompt)
	return join(lines, "\n")
}

func renderSubagentsValidation(index subagents.Index) string {
	lines := []string{fmt.Sprintf("subagents: %d", len(index.Subagents))}
	errors := 0
	for _, diagnostic := range index.Diagnostics {
		item := diagnostic.AsMap()
		lines = append(lines, fmt.Sprintf("%s: %s: %s", item["severity"], item["path"], item["message"]))
		if diagnostic.Severity == "error" {
			errors++
		}
	}
	if errors > 0 {
		lines = append(lines, fmt.Sprintf("errors: %d", errors))
	} else {
		lines = append(lines, "OK")
	}
	return join(lines, "\n")
}
