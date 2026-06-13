package subagents

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

const (
	projectSubagentsDir  = ".madharness-mini/subagents"
	maxSubagentFileBytes = 200_000
)

var (
	//go:embed prompts/subagents/*.md
	builtinPrompts embed.FS
)

type rankedSubagent struct {
	priority int
	subagent Subagent
}

// Discover читает shipped роли и project-local `.madharness-mini/subagents/*.md`.
func Discover(cfg *config.Config) Index {
	found := map[string]rankedSubagent{}
	diagnostics := []Diagnostic{}
	for _, item := range loadBuiltinSubagents() {
		diagnostics = append(diagnostics, item.diagnostics...)
		if item.ok {
			found[item.subagent.Name] = rankedSubagent{priority: 10, subagent: item.subagent}
		}
	}

	root, err := policy.New(cfg).SafePath(projectSubagentsDir)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{"error", projectSubagentsDir, err.Error()})
	} else if info, err := os.Stat(root); err == nil {
		if !info.IsDir() {
			diagnostics = append(diagnostics, Diagnostic{"error", projectSubagentsDir, "subagent root is not a directory"})
		} else {
			loadProjectSubagents(root, cfg.Root, found, &diagnostics)
		}
	} else if !os.IsNotExist(err) {
		diagnostics = append(diagnostics, Diagnostic{"error", projectSubagentsDir, err.Error()})
	}

	out := map[string]Subagent{}
	for name, item := range found {
		out[name] = item.subagent
	}
	return Index{Subagents: out, Diagnostics: diagnostics}
}

type loadResult struct {
	subagent    Subagent
	diagnostics []Diagnostic
	ok          bool
}

func loadBuiltinSubagents() []loadResult {
	entries, err := builtinPrompts.ReadDir("prompts/subagents")
	if err != nil {
		return []loadResult{{diagnostics: []Diagnostic{{"error", "internal/subagents/prompts/subagents", err.Error()}}}}
	}
	sort.Slice(entries, func(i int, j int) bool { return entries[i].Name() < entries[j].Name() })
	results := []loadResult{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := "prompts/subagents/" + entry.Name()
		location := "internal/subagents/" + path
		raw, err := builtinPrompts.ReadFile(path)
		if err != nil {
			results = append(results, loadResult{diagnostics: []Diagnostic{{"error", location, err.Error()}}})
			continue
		}
		subagent, diagnostics, ok := subagentFromText(string(raw), location, "builtin", true)
		results = append(results, loadResult{subagent: subagent, diagnostics: diagnostics, ok: ok})
	}
	return results
}

func loadProjectSubagents(root string, workspaceRoot string, found map[string]rankedSubagent, diagnostics *[]Diagnostic) {
	entries, err := os.ReadDir(root)
	if err != nil {
		*diagnostics = append(*diagnostics, Diagnostic{"error", displayPath(root, workspaceRoot), err.Error()})
		return
	}
	sort.Slice(entries, func(i int, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		subagent, items, ok := loadSubagentFile(path, workspaceRoot, "project", false)
		*diagnostics = append(*diagnostics, items...)
		if ok {
			putSubagent(found, diagnostics, subagent, 20)
		}
	}
}

func loadSubagentFile(path string, workspaceRoot string, source string, builtin bool) (Subagent, []Diagnostic, bool) {
	location := displayPath(path, workspaceRoot)
	info, err := os.Stat(path)
	if err != nil {
		return Subagent{}, []Diagnostic{{"error", location, err.Error()}}, false
	}
	if info.IsDir() {
		return Subagent{}, []Diagnostic{{"error", location, "subagent file is not a file"}}, false
	}
	if info.Size() > maxSubagentFileBytes {
		return Subagent{}, []Diagnostic{{"error", location, fmt.Sprintf("subagent file is too large: %d bytes, limit is %d", info.Size(), maxSubagentFileBytes)}}, false
	}
	if err := ensureFileInsideWorkspace(path, workspaceRoot); err != nil {
		return Subagent{}, []Diagnostic{{"error", location, err.Error()}}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Subagent{}, []Diagnostic{{"error", location, err.Error()}}, false
	}
	return subagentFromText(string(raw), location, source, builtin)
}

func subagentFromText(raw string, location string, source string, builtin bool) (Subagent, []Diagnostic, bool) {
	diagnostics := []Diagnostic{}
	fields, body, err := parseMarkdown(raw)
	if err != nil {
		return Subagent{}, []Diagnostic{{"error", location, err.Error()}}, false
	}
	name := scalar(fields["name"])
	description := scalar(fields["description"])
	profile := scalar(fields["profile"])
	if name == "" {
		diagnostics = append(diagnostics, Diagnostic{"error", location, "missing required name"})
	} else if !validName(name) {
		diagnostics = append(diagnostics, Diagnostic{"error", location, "invalid subagent name: " + name})
	}
	if description == "" {
		diagnostics = append(diagnostics, Diagnostic{"error", location, "missing required description"})
	}
	if !profileValues[profile] {
		diagnostics = append(diagnostics, Diagnostic{"error", location, "invalid profile: " + emptyLabel(profile) + "; allowed: read-only, writable"})
	}
	tools := parseTools(fields["tools"], location, &diagnostics)
	maxTurns := optionalPositiveInt(fields["max_turns"], "max_turns", location, &diagnostics)
	contextMaxTokens := optionalPositiveInt(fields["context_max_tokens"], "context_max_tokens", location, &diagnostics)
	prompt := strings.TrimSpace(body)
	if prompt == "" {
		diagnostics = append(diagnostics, Diagnostic{"error", location, "missing prompt body"})
	}
	if hasErrors(diagnostics) {
		return Subagent{}, diagnostics, false
	}
	return Subagent{
		Name:             name,
		Description:      description,
		Profile:          profile,
		Tools:            tools,
		Prompt:           prompt,
		Source:           source,
		Location:         location,
		MaxTurns:         maxTurns,
		ContextMaxTokens: contextMaxTokens,
		Metadata:         metadata(fields["metadata"]),
		Builtin:          boolField(fields["builtin"], builtin),
		Override:         boolField(fields["override"], false),
	}, diagnostics, true
}

func putSubagent(found map[string]rankedSubagent, diagnostics *[]Diagnostic, subagent Subagent, priority int) {
	current, exists := found[subagent.Name]
	if !exists {
		found[subagent.Name] = rankedSubagent{priority: priority, subagent: subagent}
		return
	}
	if current.subagent.Builtin && !subagent.Override {
		*diagnostics = append(*diagnostics, Diagnostic{"warning", subagent.Location, "subagent shadows builtin name without override: " + subagent.Name})
		return
	}
	*diagnostics = append(*diagnostics, Diagnostic{"warning", current.subagent.Location, "subagent shadowed by higher-priority copy: " + subagent.Name})
	found[subagent.Name] = rankedSubagent{priority: priority, subagent: subagent}
}

func ensureFileInsideWorkspace(path string, workspaceRoot string) error {
	resolvedRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(filepath.Clean(resolvedRoot), filepath.Clean(resolvedPath))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("subagent file escapes workspace")
	}
	return nil
}
