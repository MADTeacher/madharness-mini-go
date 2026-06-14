package tools

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/processes"
	"github.com/MADTeacher/madharness-mini-go/internal/workspace"
)

// Context хранит конфиг и policy для handlers одного запуска run.
type Context struct {
	Config                *config.Config
	Policy                *policy.Policy
	Approval              *approval.Manager
	Trace                 TraceWriter
	ResourceTracker       ResourceTracker
	Scheduler             *workspace.Scheduler
	Processes             *processes.Manager
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

// LockWorkspaceRead берёт read lock для одного или нескольких workspace paths.
func (c *Context) LockWorkspaceRead(tool string, paths ...string) func() {
	return c.lockWorkspace(tool, workspace.LockRead, paths)
}

// LockWorkspaceWrite берёт write lock для одного или нескольких workspace paths.
func (c *Context) LockWorkspaceWrite(tool string, paths ...string) func() {
	return c.lockWorkspace(tool, workspace.LockWrite, paths)
}

// LockWorkspaceExclusive берёт global lock для действий с неизвестным footprint.
func (c *Context) LockWorkspaceExclusive(tool string) func() {
	return c.lockWorkspace(tool, workspace.LockExclusive, nil)
}

func (c *Context) lockWorkspace(tool string, mode workspace.LockMode, paths []string) func() {
	if c == nil || c.Scheduler == nil {
		return func() {}
	}
	displayPaths := c.workspaceLockDisplayPaths(paths)
	started := time.Now()
	release := c.Scheduler.Acquire(mode, paths)
	waitMS := int(time.Since(started).Milliseconds())
	if c.Trace != nil {
		_ = c.Trace.Write("workspace_lock_acquired", map[string]any{
			"tool":    tool,
			"mode":    string(mode),
			"paths":   displayPaths,
			"wait_ms": waitMS,
		})
	}
	return func() {
		release()
		if c.Trace != nil {
			_ = c.Trace.Write("workspace_lock_released", map[string]any{
				"tool":  tool,
				"mode":  string(mode),
				"paths": displayPaths,
			})
		}
	}
}

func (c *Context) workspaceLockDisplayPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if c != nil && c.Config != nil {
			if rel, err := filepath.Rel(c.Config.Root, path); err == nil {
				out = append(out, filepath.ToSlash(rel))
				continue
			}
		}
		out = append(out, filepath.ToSlash(path))
	}
	return out
}

// TraceWriter — минимальный контракт trace, нужный handlers инструментов.
type TraceWriter interface {
	Write(event string, fields map[string]any) error
}

// ResourceTracker определяет, относится ли путь к активному skill.
type ResourceTracker interface {
	ResourceEvent(path string) map[string]any
}
