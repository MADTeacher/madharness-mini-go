package skills

import (
	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// CatalogProvider добавляет в run компактный список доступных skills.
type CatalogProvider struct {
	Index         Index
	WorkspaceRoot string
}

// Collect возвращает transient catalog-фрагмент, если skills найдены.
func (p CatalogProvider) Collect(agentcontext.State) []agentcontext.Fragment {
	text := RenderCatalog(p.Index, p.WorkspaceRoot)
	if text == "" {
		return nil
	}
	return []agentcontext.Fragment{{
		ID:        "skills:catalog",
		Source:    "project skill catalog",
		Text:      text,
		Priority:  30,
		Placement: "system",
		Transient: true,
	}}
}

// ToolProvider выдаёт модели activate_skill с enum найденных имён.
type ToolProvider struct {
	Runtime *Runtime
}

// Specs регистрирует activate_skill только при наличии валидных skills.
func (p ToolProvider) Specs(*tools.Context) []tools.Spec {
	if p.Runtime == nil {
		return nil
	}
	names := p.Runtime.Index().Names()
	if len(names) == 0 {
		return nil
	}
	return []tools.Spec{{
		Name:        "activate_skill",
		Description: "Activate one available Agent Skill by exact name. The harness adds the skill instructions to durable context and returns the skill root plus bundled resource list.",
		Parameters: tools.Obj(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"enum":        names,
				"description": "Exact skill name from the available Agent Skills catalog.",
			},
		}, []string{"name"}),
		Handler: func(ctx *tools.Context, args map[string]any) tools.Observation {
			_ = ctx
			return p.Runtime.Activate(tools.StringArg(args, "name", ""), "tool")
		},
	}}
}
