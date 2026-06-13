// Package tools описывает общий контракт инструментов agent loop.
package tools

// Observation — единый результат tool call, который возвращается модели.
type Observation map[string]any

// OK сообщает модели об успешном действии инструмента.
func OK(tool string, summary string, data map[string]any) Observation {
	obs := Observation{"ok": true, "tool": tool, "summary": summary}
	for key, value := range data {
		obs[key] = value
	}
	return obs
}

// Fail сообщает модели об отказе политики, ошибке аргументов или сбое handler.
func Fail(tool string, summary string, data ...map[string]any) Observation {
	obs := Observation{"ok": false, "tool": tool, "summary": summary}
	if len(data) > 0 {
		for key, value := range data[0] {
			obs[key] = value
		}
	}
	return obs
}
