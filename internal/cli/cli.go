// Package cli разбирает команды madharness-mini.
package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/MADTeacher/madharness-mini-go/internal/agent"
	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/skills"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Main запускает CLI и возвращает process exit code.
func Main(argv []string, stdout io.Writer, stderr io.Writer) int {
	return MainWithInput(argv, nil, stdout, stderr)
}

// MainWithInput запускает CLI с явным stdin для интерактивного approval prompt.
func MainWithInput(argv []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprintln(stderr, "usage: madharness-mini <init|ask|run|trace|skills|subagents> ...")
		return 2
	}
	cfg, err := config.New("")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	switch argv[0] {
	case "init":
		return runInit(argv[1:], cfg, stdout, stderr)
	case "ask":
		return runAsk(argv[1:], cfg, stdout, stderr)
	case "run":
		return runAgent(argv[1:], cfg, stdin, stdout, stderr)
	case "trace":
		return runTrace(argv[1:], cfg, stdout, stderr)
	case "skills":
		return runSkills(argv[1:], cfg, stdout, stderr)
	case "subagents":
		return runSubagents(argv[1:], cfg, stdout, stderr)
	default:
		fmt.Fprintln(stderr, "error: unknown command:", argv[0])
		return 2
	}
}

func runInit(argv []string, cfg *config.Config, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	model := fs.String("model", "", "model name")
	baseURL := fs.String("base-url", "", "OpenAI-compatible base URL")
	apiKey := fs.String("api-key", "", "API key")
	_ = fs.Bool("no-prompt", false, "kept for compatibility; Go port does not prompt")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	path, changes, err := cfg.Initialize(config.InitOptions{Model: *model, BaseURL: *baseURL, APIKey: *apiKey})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Настройка записана:", path)
	if len(changes) > 0 {
		fmt.Fprintln(stdout, "Обновлено: "+formatChanges(changes))
	}
	if cfg.Data.APIKey == "" {
		fmt.Fprintln(stdout, "Ключ API не задан. Передайте --api-key или задайте MADHARNESS_MINI_API_KEY перед запуском ask/run.")
	}
	return 0
}

func runAsk(argv []string, cfg *config.Config, stdout io.Writer, stderr io.Writer) int {
	task, ok := oneTask(argv, stderr)
	if !ok {
		return 2
	}
	result, tracePath, err := agent.Ask(task, cfg)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, result)
	fmt.Fprintln(stderr, "\nTrace:", tracePath)
	return 0
}

func runAgent(argv []string, cfg *config.Config, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runFlags := registerRunFlags(fs)
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	options, err := runFlags.options()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	task, ok := oneTask(fs.Args(), stderr)
	if !ok {
		return 2
	}
	if selectedApprovalMode(cfg, options) == approval.ModeAsk {
		options.ApprovalPrompter = approval.NewCLIPrompter(stdin, stderr)
	}
	result, tracePath, err := agent.RunWithOptions(task, cfg, options)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, result)
	fmt.Fprintln(stderr, "\nTrace:", tracePath)
	return 0
}

func selectedApprovalMode(cfg *config.Config, options agent.RunOptions) string {
	if options.ApprovalMode != "" {
		return options.ApprovalMode
	}
	return cfg.Data.ApprovalMode
}

func runTrace(argv []string, cfg *config.Config, stdout io.Writer, stderr io.Writer) int {
	if len(argv) != 1 {
		fmt.Fprintln(stderr, "usage: madharness-mini trace <trace-id>")
		return 2
	}
	result, err := trace.Summarize(cfg, argv[0])
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, result)
	return 0
}

func runSkills(argv []string, cfg *config.Config, stdout io.Writer, stderr io.Writer) int {
	if len(argv) < 1 {
		fmt.Fprintln(stderr, "usage: madharness-mini skills <list|show|validate> [name]")
		return 2
	}
	result, err := SkillsCommand(cfg, argv[0], argv[1:]...)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, result)
	return 0
}

func runSubagents(argv []string, cfg *config.Config, stdout io.Writer, stderr io.Writer) int {
	if len(argv) < 1 {
		fmt.Fprintln(stderr, "usage: madharness-mini subagents <list|show|validate> [name]")
		return 2
	}
	result, err := SubagentsCommand(cfg, argv[0], argv[1:]...)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, result)
	return 0
}

// SkillsCommand печатает найденные Agent Skills и диагностику их SKILL.md.
func SkillsCommand(cfg *config.Config, command string, args ...string) (string, error) {
	index := skills.Discover(cfg)
	switch command {
	case "list":
		if len(index.Skills) == 0 {
			return "Навыки не найдены.", nil
		}
		lines := []string{"Доступные навыки:"}
		for _, name := range index.Names() {
			skill := index.Skills[name]
			lines = append(lines, fmt.Sprintf("- %s: %s (%s)", skill.Name, skill.Description, skill.Location(cfg.Root)))
		}
		return join(lines, "\n"), nil
	case "show":
		if len(args) != 1 {
			return "", fmt.Errorf("usage: madharness-mini skills show <name>")
		}
		skill, ok := index.Skills[args[0]]
		if !ok {
			return "", fmt.Errorf("skill not found: %s", args[0])
		}
		return renderSkill(skill, cfg.Root), nil
	case "validate":
		lines := []string{fmt.Sprintf("skills: %d", len(index.Skills))}
		for _, diagnostic := range index.Diagnostics {
			item := diagnostic.AsMap(cfg.Root)
			lines = append(lines, fmt.Sprintf("%s: %s: %s", item["severity"], item["path"], item["message"]))
		}
		errors := 0
		for _, diagnostic := range index.Diagnostics {
			if diagnostic.Severity == "error" {
				errors++
			}
		}
		if errors > 0 {
			lines = append(lines, fmt.Sprintf("errors: %d", errors))
		} else {
			lines = append(lines, "OK")
		}
		return join(lines, "\n"), nil
	default:
		return "", fmt.Errorf("unknown skills command: %s", command)
	}
}

func renderSkill(skill skills.Skill, root string) string {
	lines := []string{
		"name: " + skill.Name,
		"description: " + skill.Description,
		"location: " + skill.Location(root),
		"root: " + skill.RootLocation(root),
		"source: " + skill.Source,
	}
	if skill.License != "" {
		lines = append(lines, "license: "+skill.License)
	}
	if skill.Compatibility != "" {
		lines = append(lines, "compatibility: "+skill.Compatibility)
	}
	if len(skill.AllowedTools) > 0 {
		lines = append(lines, "allowed-tools: "+join(skill.AllowedTools, " "))
	}
	if len(skill.Metadata) > 0 {
		lines = append(lines, "metadata:")
		keys := make([]string, 0, len(skill.Metadata))
		for key := range skill.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("  %s: %s", key, skill.Metadata[key]))
		}
	}
	lines = append(lines, "resources:")
	resources := skills.ListResources(skill, root)
	if len(resources) == 0 {
		lines = append(lines, "  none")
	} else {
		for _, resource := range resources {
			lines = append(lines, fmt.Sprintf("  - %s (%s, %d bytes)", resource.WorkspacePath, resource.Kind, resource.Bytes))
		}
	}
	lines = append(lines, "", "instructions:", skill.Body)
	return join(lines, "\n")
}

func oneTask(argv []string, stderr io.Writer) (string, bool) {
	if len(argv) != 1 {
		fmt.Fprintln(stderr, "usage: madharness-mini ask|run <task>")
		return "", false
	}
	return argv[0], true
}

func formatChanges(changes []string) string {
	names := map[string]string{
		"api_key":  "api_key",
		"base_url": "base_url",
		"created":  "config.json",
		"model":    "model",
	}
	seen := map[string]bool{}
	out := []string{}
	for _, change := range changes {
		name := names[change]
		if name == "" {
			name = change
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return join(out, ", ")
}

func join(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for _, item := range items[1:] {
		result += sep + item
	}
	return result
}
