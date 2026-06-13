package skills

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// Runtime хранит состояние skills одного run-запуска.
type Runtime struct {
	cfg    *config.Config
	index  Index
	active map[string]bool
}

// NewRuntime создаёт состояние активации поверх найденного индекса.
func NewRuntime(cfg *config.Config, index Index) *Runtime {
	return &Runtime{cfg: cfg, index: index, active: map[string]bool{}}
}

// Index возвращает найденные skills для providers и диагностики.
func (r *Runtime) Index() Index {
	return r.index
}

// Activate добавляет skill в durable context или сообщает, что он уже активен.
func (r *Runtime) Activate(name string, trigger string) tools.Observation {
	skill, ok := r.index.Skills[name]
	if !ok {
		return tools.Fail("activate_skill", "unknown skill: "+name, map[string]any{"name": name})
	}
	resources := ListResources(skill, r.cfg.Root)
	resourceItems := resourceMaps(resources)
	if r.active[name] {
		return tools.OK("activate_skill", "skill already active: "+name, map[string]any{
			"name":           name,
			"already_active": true,
			"skill_root":     skill.RootLocation(r.cfg.Root),
			"resources":      resourceItems,
		})
	}
	r.active[name] = true
	obs := tools.OK("activate_skill", "activated skill: "+name+"; instructions were added to durable context", map[string]any{
		"name":           name,
		"already_active": false,
		"skill_root":     skill.RootLocation(r.cfg.Root),
		"location":       skill.Location(r.cfg.Root),
		"resources":      resourceItems,
	})
	obs["_context_fragments"] = []agentcontext.Fragment{ActivationFragment(skill, r.cfg.Root, resources)}
	obs["_skill_event"] = map[string]any{
		"name":      name,
		"trigger":   trigger,
		"location":  skill.Location(r.cfg.Root),
		"resources": len(resources),
	}
	return obs
}

// ResourceEvent готовит trace-событие, если tool обратился к активному skill.
func (r *Runtime) ResourceEvent(path string) map[string]any {
	resolved := filepath.Clean(path)
	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		resolved = filepath.Clean(realPath)
	}
	names := make([]string, 0, len(r.active))
	for name := range r.active {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		skill, ok := r.index.Skills[name]
		if !ok || !pathInside(resolved, skill.Root) {
			continue
		}
		relative, err := filepath.Rel(skill.Root, resolved)
		if err != nil {
			continue
		}
		if relative == "." {
			return map[string]any{"name": name, "path": ".", "skill_root": skill.RootLocation(r.cfg.Root)}
		}
		if splitBySlash(filepath.ToSlash(relative))[0] == "SKILL.md" {
			continue
		}
		return map[string]any{
			"name":       name,
			"path":       filepath.ToSlash(relative),
			"skill_root": skill.RootLocation(r.cfg.Root),
		}
	}
	return nil
}

// ActivationFragment упаковывает полный workflow skill в system-фрагмент.
func ActivationFragment(skill Skill, workspaceRoot string, resources []Resource) agentcontext.Fragment {
	resourceLines := resourceLines(resources)
	parts := []string{
		"# Active Agent Skill: " + skill.Name,
		"",
		"Location: `" + skill.Location(workspaceRoot) + "`",
		"Skill root: `" + skill.RootLocation(workspaceRoot) + "`",
		"Description: " + skill.Description,
	}
	if skill.Compatibility != "" {
		parts = append(parts, "Compatibility: "+skill.Compatibility)
	}
	if skill.License != "" {
		parts = append(parts, "License: "+skill.License)
	}
	if len(skill.AllowedTools) > 0 {
		parts = append(parts, "Allowed tools (experimental; global harness policy still applies): "+strings.Join(skill.AllowedTools, " "))
	}
	if len(skill.Metadata) > 0 {
		items := []string{}
		for key, value := range skill.Metadata {
			items = append(items, fmt.Sprintf("%s=%s", key, value))
		}
		sort.Strings(items)
		parts = append(parts, "Metadata: "+strings.Join(items, ", "))
	}
	parts = append(parts,
		"",
		"Follow these skill instructions when they are relevant to the user's task.",
		"",
		"## Instructions",
		"",
		skillBody(skill),
		"",
		"## Bundled resources",
		"",
		resourceLines,
	)
	return agentcontext.Fragment{
		ID:        "skill:" + skill.Name,
		Source:    skill.Location(workspaceRoot),
		Text:      strings.Join(parts, "\n"),
		Priority:  20,
		Placement: "system",
		Transient: false,
	}
}

func skillBody(skill Skill) string {
	if skill.Body == "" {
		return "(SKILL.md body is empty.)"
	}
	return skill.Body
}

func resourceLines(resources []Resource) string {
	if len(resources) == 0 {
		return "No bundled resources were found."
	}
	lines := []string{}
	for _, resource := range resources {
		lines = append(lines, fmt.Sprintf("- `%s` (%s, %d bytes; workspace path: `%s`)", resource.RelativePath, resource.Kind, resource.Bytes, resource.WorkspacePath))
	}
	return strings.Join(lines, "\n")
}
