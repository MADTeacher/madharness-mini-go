package subagents

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

var requestMarkers = []string{
	"delegate_task",
	"orchestrate",
	"orchestration",
	"subagent",
	"subagents",
	"используй субагент",
	"использовать субагент",
	"субагент",
	"субагенты",
	"оркестр",
	"делегируй",
	"делегировать",
}

// ParentRequiredTools ограничивает parent agent в строгой оркестрации.
var ParentRequiredTools = []string{"list_files", "read_file", "search_code", "delegate_task"}

// Orchestration описывает фактический режим для одного run.
type Orchestration struct {
	Configured      string
	Effective       string
	Source          string
	RequestedByTask bool
}

// ResolveOrchestrationMode сводит config/env/CLI к фактическому режиму.
func ResolveOrchestrationMode(cfg *config.Config, task string, override string) (Orchestration, error) {
	source := "config"
	configured := strings.ToLower(strings.TrimSpace(cfg.Data.OrchestrationMode))
	if configured == "" {
		configured = "auto"
	}
	if override != "" {
		configured = strings.ToLower(strings.TrimSpace(override))
		source = "cli"
	}
	if !config.OrchestrationModeValues[configured] {
		return Orchestration{}, fmt.Errorf("invalid orchestration mode: %s; allowed: auto, off, requested, required", configured)
	}
	if override == "" && !cfg.Data.OrchestrationEnabled && configured == "auto" {
		configured = "off"
		source = "legacy orchestration_enabled"
	}
	requested := TaskRequestsOrchestration(task)
	effective := configured
	if configured == "requested" && !requested {
		effective = "off"
	}
	return Orchestration{
		Configured:      configured,
		Effective:       effective,
		Source:          source,
		RequestedByTask: requested,
	}, nil
}

// TaskRequestsOrchestration ищет явные маркеры делегации в задаче пользователя.
func TaskRequestsOrchestration(task string) bool {
	lowered := strings.ToLower(task)
	for _, marker := range requestMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// ParentAllowedTools возвращает allow-list parent tools для выбранного режима.
func ParentAllowedTools(mode string) []string {
	if mode == "required" {
		return append([]string{}, ParentRequiredTools...)
	}
	return nil
}

// RequiredFragment объясняет parent agent строгий режим оркестрации.
func RequiredFragment() agentcontext.Fragment {
	return agentcontext.Fragment{
		ID:        "orchestration:required",
		Source:    "runtime",
		Priority:  4,
		Placement: "system",
		Text: "Оркестрация обязательна для этого запуска. " +
			"Ведущий агент координирует работу и делегирует исследование, " +
			"планирование, реализацию, проверку или уточнение через `delegate_task`. " +
			"Не выполняй файловые правки или shell-проверки самостоятельно; поручай " +
			"их подходящим writable/read-only субагентам.",
	}
}
