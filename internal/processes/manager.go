// Package processes управляет долгоживущими subprocess-ами одного agent run.
package processes

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

const (
	bufferLimit       = 20000
	traceOutputLimit  = 4000
	defaultStopWait   = 5 * time.Second
	closeAllStopWait  = 2 * time.Second
	processIDTemplate = "proc-%d"
)

// TraceWriter — минимальный контракт записи lifecycle-событий managed process.
type TraceWriter interface {
	Write(event string, fields map[string]any) error
}

// StartOptions описывает проверенный subprocess, который можно запустить.
type StartOptions struct {
	Command      string
	Argv         []string
	CWD          string
	CWDDisplay   string
	Name         string
	Shell        bool
	ReadyPattern string
	ReadyTimeout time.Duration
	Trace        TraceWriter
}

// StopOptions задаёт, какой managed process остановить.
type StopOptions struct {
	ProcessID string
	Name      string
	Timeout   time.Duration
	Reason    string
	Trace     TraceWriter
}

// Status описывает состояние процесса для tool observation и тестов.
type Status struct {
	ProcessID string
	Name      string
	Command   string
	CWD       string
	PID       int
	Shell     bool
	Running   bool
	Ready     bool
	ExitCode  *int
	Stdout    string
	Stderr    string
}

// Manager хранит процессы, доступные root-agent и субагентам одного run.
type Manager struct {
	mu        sync.Mutex
	next      int
	processes map[string]*Process
	names     map[string]string
	closed    bool
}

// Process хранит runtime-состояние одного subprocess.
type Process struct {
	id         string
	name       string
	command    string
	cwd        string
	pid        int
	shell      bool
	cmd        *exec.Cmd
	stdout     boundedBuffer
	stderr     boundedBuffer
	readyRe    *regexp.Regexp
	readyCh    chan struct{}
	doneCh     chan struct{}
	readyOnce  sync.Once
	doneOnce   sync.Once
	waitErr    error
	exitCode   *int
	stopReason string
	ready      bool
	running    bool
	mu         sync.Mutex
}

// NewManager создаёт пустой manager для одного run.
func NewManager() *Manager {
	return &Manager{
		processes: map[string]*Process{},
		names:     map[string]string{},
	}
}

// Start запускает subprocess и при необходимости ждёт readiness по output regex.
func (m *Manager) Start(options StartOptions) (Status, error) {
	if m == nil {
		return Status{}, errors.New("process manager is not configured")
	}
	if len(options.Argv) == 0 {
		return Status{}, errors.New("empty process argv")
	}
	readyRe, err := compileReadyPattern(options.ReadyPattern)
	if err != nil {
		return Status{}, err
	}
	process, err := m.reserve(options, readyRe)
	if err != nil {
		return Status{}, err
	}
	cmd := exec.Command(options.Argv[0], options.Argv[1:]...)
	cmd.Dir = options.CWD
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.dropReserved(process)
		return Status{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.dropReserved(process)
		return Status{}, err
	}
	if err := cmd.Start(); err != nil {
		m.dropReserved(process)
		writeTrace(options.Trace, "process_failed", map[string]any{
			"process_id": process.id,
			"name":       process.name,
			"command":    process.command,
			"error":      err.Error(),
		})
		return Status{}, err
	}
	process.mu.Lock()
	process.cmd = cmd
	process.pid = cmd.Process.Pid
	process.running = true
	process.mu.Unlock()
	writeTrace(options.Trace, "process_started", map[string]any{
		"process_id": process.id,
		"name":       process.name,
		"command":    process.command,
		"cwd":        process.cwd,
		"pid":        process.pid,
	})
	go process.readStream("stdout", stdout, options.Trace)
	go process.readStream("stderr", stderr, options.Trace)
	go m.waitProcess(process, options.Trace, "exit")
	if readyRe == nil {
		return process.Status(), nil
	}
	timeout := options.ReadyTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case <-process.readyCh:
	case <-process.doneCh:
		status := process.Status()
		if !status.Ready {
			return status, ErrExitedBeforeReady
		}
	case <-time.After(timeout):
	}
	return process.Status(), nil
}

// Status возвращает состояние одного процесса или список всех известных процессов.
func (m *Manager) Status(processID string, name string) ([]Status, error) {
	if m == nil {
		return nil, errors.New("process manager is not configured")
	}
	if processID != "" || name != "" {
		process, err := m.find(processID, name)
		if err != nil {
			return nil, err
		}
		return []Status{process.Status()}, nil
	}
	m.mu.Lock()
	processes := make([]*Process, 0, len(m.processes))
	for _, process := range m.processes {
		processes = append(processes, process)
	}
	m.mu.Unlock()
	out := make([]Status, 0, len(processes))
	for _, process := range processes {
		out = append(out, process.Status())
	}
	return out, nil
}

// Stop мягко завершает процесс и при необходимости убивает его.
func (m *Manager) Stop(options StopOptions) (Status, error) {
	if m == nil {
		return Status{}, errors.New("process manager is not configured")
	}
	process, err := m.find(options.ProcessID, options.Name)
	if err != nil {
		return Status{}, err
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultStopWait
	}
	reason := options.Reason
	if reason == "" {
		reason = "stop_shell"
	}
	return process.stop(timeout, reason), nil
}

// CloseAll завершает все живые процессы перед выходом из run.
func (m *Manager) CloseAll(trace TraceWriter) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	processes := make([]*Process, 0, len(m.processes))
	for _, process := range m.processes {
		if process.Status().Running {
			processes = append(processes, process)
		}
	}
	m.mu.Unlock()
	for _, process := range processes {
		_ = process.stop(closeAllStopWait, "run_cleanup")
	}
}

// ErrExitedBeforeReady означает, что процесс завершился до ready_pattern.
var ErrExitedBeforeReady = errors.New("process exited before readiness")

func (m *Manager) reserve(options StartOptions, readyRe *regexp.Regexp) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("process manager is closed")
	}
	if options.Name != "" {
		if _, exists := m.names[options.Name]; exists {
			return nil, fmt.Errorf("managed process name already running: %s", options.Name)
		}
	}
	m.next++
	id := fmt.Sprintf(processIDTemplate, m.next)
	process := &Process{
		id:      id,
		name:    options.Name,
		command: options.Command,
		cwd:     options.CWDDisplay,
		shell:   options.Shell,
		readyRe: readyRe,
		readyCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
	m.processes[id] = process
	if options.Name != "" {
		m.names[options.Name] = id
	}
	return process, nil
}

func (m *Manager) dropReserved(process *Process) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.processes, process.id)
	if process.name != "" {
		delete(m.names, process.name)
	}
}

func (m *Manager) waitProcess(process *Process, trace TraceWriter, reason string) {
	err := process.cmd.Wait()
	process.finish(err)
	m.mu.Lock()
	if process.name != "" && m.names[process.name] == process.id {
		delete(m.names, process.name)
	}
	m.mu.Unlock()
	status := process.Status()
	stopReason := process.stopReasonOr(reason)
	writeTrace(trace, "process_stopped", map[string]any{
		"process_id": status.ProcessID,
		"name":       status.Name,
		"exit_code":  exitCodeValue(status.ExitCode),
		"reason":     stopReason,
	})
}

func (m *Manager) find(processID string, name string) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if processID != "" {
		process, ok := m.processes[processID]
		if !ok {
			return nil, fmt.Errorf("managed process not found: %s", processID)
		}
		return process, nil
	}
	if name != "" {
		id, ok := m.names[name]
		if !ok {
			for _, process := range m.processes {
				if process.name == name {
					return process, nil
				}
			}
			return nil, fmt.Errorf("managed process not found by name: %s", name)
		}
		return m.processes[id], nil
	}
	return nil, errors.New("process_id or name is required")
}

func (p *Process) readStream(stream string, reader io.Reader, trace TraceWriter) {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			text := string(buf[:n])
			p.appendOutput(stream, text)
			writeTrace(trace, "process_output", map[string]any{
				"process_id": p.id,
				"name":       p.name,
				"stream":     stream,
				"content":    clipped(text, traceOutputLimit),
			})
		}
		if err != nil {
			return
		}
	}
}

func (p *Process) appendOutput(stream string, text string) {
	p.mu.Lock()
	if stream == "stderr" {
		p.stderr.Append(text)
	} else {
		p.stdout.Append(text)
	}
	if !p.ready && p.readyRe != nil && p.readyRe.MatchString(p.stdout.String()+"\n"+p.stderr.String()) {
		p.ready = true
		p.readyOnce.Do(func() { close(p.readyCh) })
	}
	p.mu.Unlock()
}

func (p *Process) finish(err error) {
	p.mu.Lock()
	p.waitErr = err
	p.running = false
	if p.cmd != nil && p.cmd.ProcessState != nil {
		code := p.cmd.ProcessState.ExitCode()
		p.exitCode = &code
	}
	p.doneOnce.Do(func() { close(p.doneCh) })
	p.mu.Unlock()
}

func (p *Process) stop(timeout time.Duration, reason string) Status {
	p.mu.Lock()
	running := p.running
	cmd := p.cmd
	done := p.doneCh
	if reason != "" && p.stopReason == "" {
		p.stopReason = reason
	}
	p.mu.Unlock()
	if !running || cmd == nil || cmd.Process == nil {
		return p.Status()
	}
	_ = cmd.Process.Signal(os.Interrupt)
	select {
	case <-done:
		return p.Status()
	case <-time.After(timeout):
	}
	_ = cmd.Process.Kill()
	select {
	case <-done:
	case <-time.After(timeout):
	}
	return p.Status()
}

func (p *Process) stopReasonOr(fallback string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopReason != "" {
		return p.stopReason
	}
	return fallback
}

// Status строит потокобезопасный snapshot процесса.
func (p *Process) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Status{
		ProcessID: p.id,
		Name:      p.name,
		Command:   p.command,
		CWD:       p.cwd,
		PID:       p.pid,
		Shell:     p.shell,
		Running:   p.running,
		Ready:     p.ready,
		ExitCode:  cloneExitCode(p.exitCode),
		Stdout:    p.stdout.String(),
		Stderr:    p.stderr.String(),
	}
}

type boundedBuffer struct {
	buf bytes.Buffer
}

func (b *boundedBuffer) Append(text string) {
	if text == "" {
		return
	}
	b.buf.WriteString(text)
	if b.buf.Len() <= bufferLimit {
		return
	}
	raw := b.buf.Bytes()
	keep := append([]byte{}, raw[len(raw)-bufferLimit:]...)
	b.buf.Reset()
	b.buf.Write(keep)
}

func (b *boundedBuffer) String() string {
	return b.buf.String()
}

func compileReadyPattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid ready_pattern: %w", err)
	}
	return compiled, nil
}

func cloneExitCode(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func exitCodeValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func writeTrace(trace TraceWriter, event string, fields map[string]any) {
	if trace == nil {
		return
	}
	_ = trace.Write(event, fields)
}

func clipped(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + fmt.Sprintf("\n...[clipped %d chars]", len(text)-limit)
}
