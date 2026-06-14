package tools

// Effect описывает класс побочного эффекта tool для планировщика turn-а.
type Effect string

const (
	// EffectExclusive сохраняет последовательное выполнение для неизвестных tools.
	EffectExclusive Effect = "exclusive"
	// EffectRead помечает tool без намеренных изменений workspace или состояния run.
	EffectRead Effect = "read"
	// EffectWrite помечает tool, который меняет файлы workspace.
	EffectWrite Effect = "write"
	// EffectShell помечает subprocess-tool с неизвестными побочными эффектами.
	EffectShell Effect = "shell"
	// EffectState помечает tool, который меняет состояние harness-сессии.
	EffectState Effect = "state"
	// EffectDelegate помечает запуск дочернего агентского цикла.
	EffectDelegate Effect = "delegate"
	// EffectMCP помечает внешний MCP tool поверх stdio transport.
	EffectMCP Effect = "mcp"
)

// NormalizeEffect превращает пустой effect старых specs в безопасный barrier.
func NormalizeEffect(effect Effect) Effect {
	if effect == "" {
		return EffectExclusive
	}
	return effect
}

// Handler выполняет конкретный инструмент в общем контексте запуска.
type Handler func(*Context, map[string]any) Observation

// Provider отдаёт набор инструментов для registry.
type Provider interface {
	Specs(*Context) ([]Spec, error)
}

// CloseProvider освобождает ресурсы provider-а после agent run.
type CloseProvider interface {
	Close(TraceWriter)
}

// Spec описывает один tool: имя, описание, JSON Schema и handler.
type Spec struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     Handler
	Effect      Effect
}

// Schema упаковывает инструмент в формат Chat Completions API.
func (s Spec) Schema() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        s.Name,
			"description": s.Description,
			"parameters":  s.Parameters,
		},
	}
}
