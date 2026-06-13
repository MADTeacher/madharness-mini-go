package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

const maxSkillFileBytes = 200_000

var skillRoots = []struct {
	path     string
	source   string
	priority int
}{
	{path: ".agents/skills", source: "agents", priority: 10},
	{path: ".madharness_mini/skills", source: "native", priority: 20},
}

type rankedSkill struct {
	priority int
	skill    Skill
}

// Discover сканирует project-local каталоги skills внутри workspace.
func Discover(cfg *config.Config) Index {
	pol := policy.New(cfg)
	found := map[string]rankedSkill{}
	diagnostics := []Diagnostic{}
	for _, rootSpec := range skillRoots {
		root, err := pol.SkillRoot(rootSpec.path)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{"error", filepath.Join(cfg.Root, rootSpec.path), err.Error()})
			continue
		}
		info, err := os.Stat(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{"error", root, err.Error()})
			continue
		}
		if !info.IsDir() {
			diagnostics = append(diagnostics, Diagnostic{"error", root, "skill root is not a directory"})
			continue
		}
		children, err := os.ReadDir(root)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{"error", root, err.Error()})
			continue
		}
		sort.Slice(children, func(i int, j int) bool { return children[i].Name() < children[j].Name() })
		for _, child := range children {
			if !child.IsDir() {
				continue
			}
			skill, diags, ok := loadSkill(filepath.Join(root, child.Name()), cfg.Root, rootSpec.source)
			diagnostics = append(diagnostics, diags...)
			if !ok {
				continue
			}
			current, exists := found[skill.Name]
			if exists && current.priority > rootSpec.priority {
				diagnostics = append(diagnostics, Diagnostic{
					Severity: "warning",
					Path:     skill.SkillFile,
					Message:  "skill shadowed by higher-priority copy: " + skill.Name,
				})
				continue
			}
			if exists {
				diagnostics = append(diagnostics, Diagnostic{
					Severity: "warning",
					Path:     current.skill.SkillFile,
					Message:  "skill shadowed by higher-priority copy: " + skill.Name,
				})
			}
			found[skill.Name] = rankedSkill{priority: rootSpec.priority, skill: skill}
		}
	}
	out := map[string]Skill{}
	for name, item := range found {
		out[name] = item.skill
	}
	return Index{Skills: out, Diagnostics: diagnostics}
}

func loadSkill(root string, workspaceRoot string, source string) (Skill, []Diagnostic, bool) {
	diagnostics := []Diagnostic{}
	resolvedWorkspace := filepath.Clean(workspaceRoot)
	if realRoot, err := filepath.EvalSymlinks(workspaceRoot); err == nil {
		resolvedWorkspace = filepath.Clean(realRoot)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Skill{}, []Diagnostic{{"error", root, "cannot resolve skill directory: " + err.Error()}}, false
	}
	if !pathInside(resolvedRoot, resolvedWorkspace) {
		return Skill{}, []Diagnostic{{"error", root, "skill directory escapes workspace"}}, false
	}
	skillFile := filepath.Join(root, "SKILL.md")
	info, err := os.Stat(skillFile)
	if os.IsNotExist(err) {
		return Skill{}, nil, false
	}
	if err != nil {
		return Skill{}, []Diagnostic{{"error", skillFile, err.Error()}}, false
	}
	if info.IsDir() {
		return Skill{}, []Diagnostic{{"error", skillFile, "SKILL.md is not a file"}}, false
	}
	resolvedSkillFile, err := filepath.EvalSymlinks(skillFile)
	if err != nil {
		return Skill{}, []Diagnostic{{"error", skillFile, "cannot resolve SKILL.md: " + err.Error()}}, false
	}
	if !pathInside(resolvedSkillFile, resolvedRoot) {
		return Skill{}, []Diagnostic{{"error", skillFile, "SKILL.md escapes skill root"}}, false
	}
	if info.Size() > maxSkillFileBytes {
		return Skill{}, []Diagnostic{{
			Severity: "error",
			Path:     skillFile,
			Message:  fmt.Sprintf("SKILL.md is too large: %d bytes, limit is %d", info.Size(), maxSkillFileBytes),
		}}, false
	}
	raw, err := os.ReadFile(resolvedSkillFile)
	if err != nil {
		return Skill{}, []Diagnostic{{"error", skillFile, err.Error()}}, false
	}
	fields, body, err := parseSkillMarkdown(string(raw))
	if err != nil {
		return Skill{}, []Diagnostic{{"error", skillFile, err.Error()}}, false
	}
	name := scalar(fields["name"])
	description := scalar(fields["description"])
	if name == "" {
		diagnostics = append(diagnostics, Diagnostic{"error", skillFile, "missing required name"})
	} else if !validSkillName(name) {
		diagnostics = append(diagnostics, Diagnostic{"error", skillFile, "invalid skill name: " + name})
	}
	if description == "" {
		diagnostics = append(diagnostics, Diagnostic{"error", skillFile, "missing required description"})
	} else if len(description) > 1024 {
		diagnostics = append(diagnostics, Diagnostic{"warning", skillFile, "description is longer than 1024 characters"})
	}
	if hasErrors(diagnostics) {
		return Skill{}, diagnostics, false
	}
	warnings := []string{}
	if name != filepath.Base(root) {
		message := fmt.Sprintf("name does not match parent directory: %s != %s", name, filepath.Base(root))
		warnings = append(warnings, message)
		diagnostics = append(diagnostics, Diagnostic{"warning", skillFile, message})
	}
	compatibility := scalar(fields["compatibility"])
	if len(compatibility) > 500 {
		diagnostics = append(diagnostics, Diagnostic{"warning", skillFile, "compatibility is longer than 500 characters"})
	}
	return Skill{
		Name:          name,
		Description:   description,
		Root:          filepath.Clean(resolvedRoot),
		SkillFile:     filepath.Clean(resolvedSkillFile),
		Body:          strings.TrimSpace(body),
		RawText:       string(raw),
		Source:        source,
		License:       scalar(fields["license"]),
		Compatibility: compatibility,
		Metadata:      metadata(fields["metadata"]),
		AllowedTools:  strings.Fields(scalar(fields["allowed-tools"])),
		Warnings:      warnings,
	}, diagnostics, true
}

func hasErrors(diagnostics []Diagnostic) bool {
	for _, item := range diagnostics {
		if item.Severity == "error" {
			return true
		}
	}
	return false
}

func pathInside(path string, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
