package skills

import (
	"regexp"
	"strings"
)

var (
	markerRE = regexp.MustCompile(`@skill[:/]([a-z0-9][a-z0-9-]{0,63})`)
	dollarRE = regexp.MustCompile(`(^|[^\w-])\$([a-z0-9][a-z0-9-]{0,63})([^\w-]|$)`)
	phraseRE = regexp.MustCompile(`(?i)(?:используй|использовать|активируй|активировать|use|activate)\s+(?:навык|skill)\s+([a-z0-9][a-z0-9-]{0,63})`)
)

// Selection хранит явные skill-маркеры пользователя.
type Selection struct {
	Names   []string
	Unknown []string
}

// Present отличает явный выбор от обычной задачи без skill-маркеров.
func (s Selection) Present() bool {
	return len(s.Names) > 0 || len(s.Unknown) > 0
}

// FindExplicitSelection ищет @skill:name, @skill/name, $name и фразы выбора.
func FindExplicitSelection(task string, available map[string]bool) Selection {
	requested := []string{}
	unknown := []string{}
	for _, match := range markerRE.FindAllStringSubmatch(task, -1) {
		addRequested(match[1], available, &requested, &unknown)
	}
	for _, match := range phraseRE.FindAllStringSubmatch(task, -1) {
		addRequested(match[1], available, &requested, &unknown)
	}
	for _, match := range dollarRE.FindAllStringSubmatch(task, -1) {
		name := match[2]
		if available[name] || strings.Contains(name, "-") {
			addRequested(name, available, &requested, &unknown)
		}
	}
	return Selection{Names: requested, Unknown: unknown}
}

// RenderCatalog строит компактный catalog-фрагмент без тела SKILL.md.
func RenderCatalog(index Index, workspaceRoot string) string {
	if len(index.Skills) == 0 {
		return ""
	}
	lines := []string{
		"# Available Agent Skills",
		"",
		"These project-local skills are available in this `run` session. Use `activate_skill` with an exact `name` to activate a skill when relevant. Do not assume a skill is active until it has been activated.",
	}
	for _, name := range index.Names() {
		skill := index.Skills[name]
		lines = append(lines, "- `"+skill.Name+"`: "+skill.Description+" (location: `"+skill.Location(workspaceRoot)+"`)")
	}
	return strings.Join(lines, "\n")
}

func addRequested(name string, available map[string]bool, requested *[]string, unknown *[]string) {
	target := unknown
	if available[name] {
		target = requested
	}
	for _, item := range *target {
		if item == name {
			return
		}
	}
	*target = append(*target, name)
}
