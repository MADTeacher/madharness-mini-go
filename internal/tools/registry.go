package tools

import (
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

// Registry хранит инструменты в стабильном порядке и вызывает handlers по имени.
type Registry struct {
	ctx   *Context
	order []string
	tools map[string]Spec
}

// NewRegistry собирает tools от providers и проверяет дубликаты имён.
func NewRegistry(cfg *config.Config, providers ...Provider) (*Registry, error) {
	ctx := &Context{Config: cfg, Policy: policy.New(cfg)}
	registry := &Registry{ctx: ctx, tools: map[string]Spec{}}
	for _, provider := range providers {
		for _, tool := range provider.Specs(ctx) {
			if _, exists := registry.tools[tool.Name]; exists {
				return nil, fmt.Errorf("duplicate tool name: %s", tool.Name)
			}
			registry.order = append(registry.order, tool.Name)
			registry.tools[tool.Name] = tool
		}
	}
	return registry, nil
}

// Schemas возвращает описание tools для model call.
func (r *Registry) Schemas() []map[string]any {
	schemas := make([]map[string]any, 0, len(r.order))
	for _, name := range r.order {
		schemas = append(schemas, r.tools[name].Schema())
	}
	return schemas
}

// Tools возвращает зарегистрированные имена в порядке выдачи модели.
func (r *Registry) Tools() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Call выполняет handler или возвращает fail-наблюдение для неизвестного tool.
func (r *Registry) Call(name string, args map[string]any) Observation {
	tool, ok := r.tools[name]
	if !ok {
		return Fail(name, "unknown tool")
	}
	return recoverCall(name, func() Observation {
		return tool.Handler(r.ctx, args)
	})
}

func recoverCall(name string, call func() Observation) (obs Observation) {
	defer func() {
		if err := recover(); err != nil {
			obs = Fail(name, fmt.Sprintf("%T: %v", err, err))
		}
	}()
	return call()
}
