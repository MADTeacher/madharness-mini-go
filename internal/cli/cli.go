// Package cli разбирает команды madharness-mini.
package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/MADTeacher/madharness-mini-go/internal/agent"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Main запускает CLI и возвращает process exit code.
func Main(argv []string, stdout io.Writer, stderr io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprintln(stderr, "usage: madharness-mini <init|ask|run|trace> ...")
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
		return runAgent(argv[1:], cfg, stdout, stderr)
	case "trace":
		return runTrace(argv[1:], cfg, stdout, stderr)
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

func runAgent(argv []string, cfg *config.Config, stdout io.Writer, stderr io.Writer) int {
	task, ok := oneTask(argv, stderr)
	if !ok {
		return 2
	}
	result, tracePath, err := agent.Run(task, cfg)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, result)
	fmt.Fprintln(stderr, "\nTrace:", tracePath)
	return 0
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
