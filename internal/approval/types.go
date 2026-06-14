// Package approval обрабатывает явное подтверждение эскалируемых policy-отказов.
package approval

const (
	// ModeAsk просит пользователя подтвердить действие через внешний prompter.
	ModeAsk = "ask"
	// ModeDeny оставляет эскалируемый отказ заблокированным без вопроса.
	ModeDeny = "deny"
)

const (
	// SourceConfig означает отказ настройками harness без интерактивного вопроса.
	SourceConfig = "config"
	// SourceCLI означает решение пользователя в CLI.
	SourceCLI = "cli"
	// SourceHook означает блокировку lifecycle hook-ом.
	SourceHook = "hook"
	// SourceYOLO означает автоматическое разрешение workspace YOLO-режимом.
	SourceYOLO = "yolo"
)

// Request описывает одно действие, которое policy запретила, но может эскалировать.
type Request struct {
	Tool    string
	Action  string
	Subject string
	Code    string
	Reason  string
	Details map[string]any
}

// Decision фиксирует итог обработки approval request.
type Decision struct {
	Approved bool
	Source   string
	Reason   string
	Message  string
}

// Prompter спрашивает пользователя или тестовый double о конкретном действии.
type Prompter interface {
	Prompt(Request) (Decision, error)
}

func (r Request) data() map[string]any {
	data := map[string]any{
		"tool":    r.Tool,
		"action":  r.Action,
		"subject": r.Subject,
		"code":    r.Code,
		"reason":  r.Reason,
	}
	for key, value := range r.Details {
		if _, exists := data[key]; !exists {
			data[key] = value
		}
	}
	return data
}

func (d Decision) data(request Request) map[string]any {
	data := request.data()
	data["approved"] = d.Approved
	data["source"] = d.Source
	data["decision_reason"] = d.Reason
	data["message"] = d.Message
	return data
}
