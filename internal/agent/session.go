package agent

import (
	"sync"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
	"github.com/MADTeacher/madharness-mini-go/internal/workspace"
)

// Session собирает состояние одного агентского loop-а: root или subagent.
type Session struct {
	shared *sessionShared
	trace  *trace.Trace
	events *events.Bus
	kind   string
}

type sessionShared struct {
	cfg              *config.Config
	client           chatClient
	options          RunOptions
	scheduler        *workspace.Scheduler
	approvalPromptMu sync.Mutex
}

type guardedPrompter struct {
	mu    *sync.Mutex
	inner approval.Prompter
}

func newSessionShared(cfg *config.Config, client chatClient, options RunOptions) *sessionShared {
	return &sessionShared{
		cfg:       cfg,
		client:    client,
		options:   options,
		scheduler: workspace.NewScheduler(),
	}
}

func newSession(shared *sessionShared, tr *trace.Trace, eventBus *events.Bus, kind string) *Session {
	return &Session{shared: shared, trace: tr, events: eventBus, kind: kind}
}

func (s *Session) RunModelLoop(context *agentcontext.Manager, registry *tools.Registry, maxTurns int, options loopOptions) (loopResult, error) {
	if options.Events == nil {
		options.Events = s.events
	}
	if options.Kind == "" {
		options.Kind = s.kind
	}
	if options.MaxParallelToolCalls == 0 {
		options.MaxParallelToolCalls = resolvedMaxParallelToolCalls(s.shared.cfg, s.shared.options)
	}
	if options.MaxParallelSubagents == 0 {
		options.MaxParallelSubagents = resolvedMaxParallelSubagents(s.shared.cfg, s.shared.options)
	}
	return runModelLoop(s.shared.client, s.trace, context, registry, maxTurns, options)
}

func (s *Session) approvalManager(kind string) *approval.Manager {
	mode := s.shared.cfg.Data.ApprovalMode
	if s.shared.options.ApprovalMode != "" {
		mode = s.shared.options.ApprovalMode
	}
	yolo := s.shared.cfg.Data.YoloMode || s.shared.options.YoloMode
	return &approval.Manager{
		Mode:     mode,
		Yolo:     yolo,
		Kind:     kind,
		Events:   s.events,
		Prompter: s.guardedPrompter(),
	}
}

func (s *Session) guardedPrompter() approval.Prompter {
	if s.shared.options.ApprovalPrompter == nil {
		return nil
	}
	return guardedPrompter{mu: &s.shared.approvalPromptMu, inner: s.shared.options.ApprovalPrompter}
}

func (p guardedPrompter) Prompt(request approval.Request) (approval.Decision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.inner.Prompt(request)
}
