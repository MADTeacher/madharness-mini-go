package tools

import (
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

// Context хранит конфиг и policy для handlers одного запуска run.
type Context struct {
	Config          *config.Config
	Policy          *policy.Policy
	Trace           TraceWriter
	ResourceTracker ResourceTracker
}

// TraceWriter — минимальный контракт trace, нужный handlers инструментов.
type TraceWriter interface {
	Write(event string, fields map[string]any) error
}

// ResourceTracker определяет, относится ли путь к активному skill.
type ResourceTracker interface {
	ResourceEvent(path string) map[string]any
}
