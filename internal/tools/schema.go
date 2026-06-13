package tools

// Obj собирает JSON Schema object для аргументов инструмента.
func Obj(props map[string]any, required []string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

// StrParam описывает строковый параметр в JSON Schema.
func StrParam(defaultValue string, description string, required bool) map[string]any {
	data := map[string]any{"type": "string", "description": description}
	if defaultValue != "" && !required {
		data["default"] = defaultValue
	}
	return data
}

// IntParam описывает целочисленный параметр со значением по умолчанию.
func IntParam(defaultValue int) map[string]any {
	return map[string]any{"type": "integer", "default": defaultValue}
}
