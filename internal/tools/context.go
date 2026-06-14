package tools

import (
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

// Context хранит конфиг и policy для handlers одного запуска run.
type Context struct {
	Config                *config.Config
	Policy                *policy.Policy
	Approval              *approval.Manager
	Trace                 TraceWriter
	ResourceTracker       ResourceTracker
	WritableSuffixes      []string
	WriteScopeDescription string
}

// WritePathError проверяет дополнительный scope записи для специализированных запусков.
func (c *Context) WritePathError(raw string) string {
	if c == nil || len(c.WritableSuffixes) == 0 {
		return ""
	}
	lowered := strings.ToLower(raw)
	for _, suffix := range c.WritableSuffixes {
		if suffix != "" && strings.HasSuffix(lowered, suffix) {
			return ""
		}
	}
	description := c.WriteScopeDescription
	if description == "" {
		description = "this agent may write only files with allowed suffixes: " + strings.Join(c.WritableSuffixes, ", ")
	}
	return "write path denied by scope: " + description + ": " + raw
}

// TraceWriter — минимальный контракт trace, нужный handlers инструментов.
type TraceWriter interface {
	Write(event string, fields map[string]any) error
}

// ResourceTracker определяет, относится ли путь к активному skill.
type ResourceTracker interface {
	ResourceEvent(path string) map[string]any
}
