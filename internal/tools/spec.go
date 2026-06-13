package tools

// Handler выполняет конкретный инструмент в общем контексте запуска.
type Handler func(*Context, map[string]any) Observation

// Provider отдаёт набор инструментов для registry.
type Provider interface {
	Specs(*Context) []Spec
}

// Spec описывает один tool: имя, описание, JSON Schema и handler.
type Spec struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     Handler
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
