package tools

import (
	"fmt"
	"sync"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/workspace"
)

// Registry хранит инструменты в стабильном порядке и вызывает handlers по имени.
type Registry struct {
	ctx       *Context
	order     []string
	tools     map[string]Spec
	providers []Provider
	closeOnce sync.Once
}

// RegistryOptions передаёт handlers наблюдаемость текущего run.
type RegistryOptions struct {
	Approval              *approval.Manager
	Trace                 TraceWriter
	ResourceTracker       ResourceTracker
	Scheduler             *workspace.Scheduler
	AllowedTools          []string
	WritableSuffixes      []string
	WriteScopeDescription string
}

// NewRegistry собирает tools от providers и проверяет дубликаты имён.
func NewRegistry(cfg *config.Config, providers ...Provider) (*Registry, error) {
	return NewRegistryWithOptions(cfg, RegistryOptions{}, providers...)
}

// NewRegistryWithOptions собирает registry с trace и skill runtime для handlers.
func NewRegistryWithOptions(cfg *config.Config, options RegistryOptions, providers ...Provider) (*Registry, error) {
	ctx := &Context{
		Config:                cfg,
		Policy:                policy.New(cfg),
		Approval:              options.Approval,
		Trace:                 options.Trace,
		ResourceTracker:       options.ResourceTracker,
		Scheduler:             options.Scheduler,
		WritableSuffixes:      normalizeSuffixes(options.WritableSuffixes),
		WriteScopeDescription: options.WriteScopeDescription,
	}
	registry := &Registry{
		ctx:       ctx,
		tools:     map[string]Spec{},
		providers: append([]Provider{}, providers...),
	}
	allowed := allowedSet(options.AllowedTools)
	for _, provider := range registry.providers {
		specs, err := provider.Specs(ctx)
		if err != nil {
			registry.Close()
			return nil, err
		}
		for _, tool := range specs {
			if allowed != nil && !allowed[tool.Name] {
				continue
			}
			if _, exists := registry.tools[tool.Name]; exists {
				registry.Close()
				return nil, fmt.Errorf("duplicate tool name: %s", tool.Name)
			}
			registry.order = append(registry.order, tool.Name)
			registry.tools[tool.Name] = tool
		}
	}
	return registry, nil
}

func allowedSet(names []string) map[string]bool {
	if names == nil {
		return nil
	}
	out := map[string]bool{}
	for _, name := range names {
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func normalizeSuffixes(values []string) []string {
	if values == nil {
		return nil
	}
	out := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Close освобождает ресурсы provider-ов, которые живут дольше одного вызова tool.
func (r *Registry) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		for _, provider := range r.providers {
			closer, ok := provider.(CloseProvider)
			if !ok {
				continue
			}
			closer.Close(r.ctx.Trace)
		}
	})
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

// Effect возвращает класс побочного эффекта tool для планировщика turn-а.
func (r *Registry) Effect(name string) Effect {
	tool, ok := r.tools[name]
	if !ok {
		return EffectExclusive
	}
	return NormalizeEffect(tool.Effect)
}

// Call выполняет handler или возвращает fail-наблюдение для неизвестного tool.
func (r *Registry) Call(name string, args map[string]any) Observation {
	obs, _ := r.CallWithFollowups(name, args)
	return obs
}

// CallWithFollowups выполняет tool и отделяет скрытые сообщения от observation.
func (r *Registry) CallWithFollowups(name string, args map[string]any) (Observation, []map[string]any) {
	tool, ok := r.tools[name]
	if !ok {
		return Fail(name, "unknown tool"), nil
	}
	obs := recoverCall(name, func() Observation {
		return tool.Handler(r.ctx, args)
	})
	return splitFollowupMessages(obs)
}

func recoverCall(name string, call func() Observation) (obs Observation) {
	defer func() {
		if err := recover(); err != nil {
			obs = Fail(name, fmt.Sprintf("%T: %v", err, err))
		}
	}()
	return call()
}

func splitFollowupMessages(obs Observation) (Observation, []map[string]any) {
	raw, ok := obs["_followup_messages"]
	if !ok {
		return obs, nil
	}
	delete(obs, "_followup_messages")
	switch messages := raw.(type) {
	case []map[string]any:
		return obs, messages
	case []any:
		out := []map[string]any{}
		for _, item := range messages {
			if message, ok := item.(map[string]any); ok {
				out = append(out, message)
			}
		}
		return obs, out
	default:
		return obs, nil
	}
}
