package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type incomingMessage struct {
	message map[string]any
	err     error
}

// StdioClient управляет одним MCP subprocess и JSON-RPC обменом по stdout/stdin.
type StdioClient struct {
	config ServerConfig
	rpc    *JSONRPCBuilder

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	waitDone chan struct{}
	waitErr  error

	stderrMu  sync.Mutex
	stderrLog []string
	readers   sync.WaitGroup
	rpcMu     sync.Mutex
	pendingMu sync.Mutex
	pending   map[int64]chan incomingMessage
	transport error
	writeMu   sync.Mutex
	closeMu   sync.Mutex
	closeCode *int
	closed    bool
}

// NewStdioClient создаёт transport-клиент для уже проверенного MCP-сервера.
func NewStdioClient(config ServerConfig) *StdioClient {
	return &StdioClient{
		config:   config,
		rpc:      NewJSONRPCBuilder(),
		pending:  map[int64]chan incomingMessage{},
		waitDone: make(chan struct{}),
	}
}

// Config возвращает проверенные настройки сервера.
func (c *StdioClient) Config() ServerConfig {
	return c.config
}

// Start запускает сервер, проходит initialize и возвращает tools/list.
func (c *StdioClient) Start() ([]map[string]any, error) {
	cmd := exec.Command(c.config.Command, c.config.Args...)
	cmd.Dir = c.config.CWD
	cmd.Env = serverEnv(c.config.Env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.stderr = stderr
	c.startReaders()
	go c.waitForExit()

	init, err := c.Request("initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "madharness-mini-go",
			"version": "0.1.0",
		},
	})
	if err != nil {
		return nil, err
	}
	if version, _ := init["protocolVersion"].(string); version != ProtocolVersion {
		return nil, fmt.Errorf("unsupported MCP protocolVersion: %v; expected %s", init["protocolVersion"], ProtocolVersion)
	}
	if err := c.Notify("notifications/initialized", map[string]any{}); err != nil {
		return nil, err
	}
	listed, err := c.Request("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return toolsFromResult(listed)
}

// Request отправляет JSON-RPC request и ждёт response с тем же id.
func (c *StdioClient) Request(method string, params map[string]any) (map[string]any, error) {
	message := c.nextRequest(method, params)
	expectedID, _ := message["id"].(int64)
	response, err := c.addPending(expectedID)
	if err != nil {
		return nil, err
	}
	if err := c.send(message); err != nil {
		c.removePending(expectedID)
		return nil, err
	}
	timer := time.NewTimer(c.config.Timeout)
	defer timer.Stop()
	select {
	case incoming := <-response:
		if incoming.err != nil {
			return nil, incoming.err
		}
		return ParseResponse(incoming.message, expectedID)
	case <-timer.C:
		c.removePending(expectedID)
		return nil, fmt.Errorf("MCP request timed out: %s; stderr: %s", method, c.StderrExcerpt(2000))
	}
}

// Notify отправляет JSON-RPC notification без ожидания ответа.
func (c *StdioClient) Notify(method string, params map[string]any) error {
	return c.send(c.rpc.Notification(method, params))
}

// CallTool вызывает исходное имя MCP tool на сервере.
func (c *StdioClient) CallTool(name string, arguments map[string]any) (map[string]any, error) {
	return c.Request("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
}

// Close закрывает subprocess через stdin, мягкий сигнал и принудительное завершение.
func (c *StdioClient) Close() *int {
	c.closeMu.Lock()
	if c.closed {
		code := c.closeCode
		c.closeMu.Unlock()
		return code
	}
	c.closed = true
	c.closeMu.Unlock()
	c.failPending(fmt.Errorf("MCP transport closed for server %s", c.config.Name))

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	c.closeStdin()
	code := c.waitOrSignal()
	c.closePipes()
	c.waitReaders()

	c.closeMu.Lock()
	c.closeCode = code
	c.closeMu.Unlock()
	return code
}

func (c *StdioClient) nextRequest(method string, params map[string]any) map[string]any {
	c.rpcMu.Lock()
	defer c.rpcMu.Unlock()
	return c.rpc.Request(method, params)
}

func (c *StdioClient) addPending(id int64) (<-chan incomingMessage, error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.transport != nil {
		return nil, c.transport
	}
	response := make(chan incomingMessage, 1)
	c.pending[id] = response
	return response, nil
}

func (c *StdioClient) removePending(id int64) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *StdioClient) completePending(id int64, incoming incomingMessage) {
	c.pendingMu.Lock()
	response := c.pending[id]
	delete(c.pending, id)
	c.pendingMu.Unlock()
	if response != nil {
		response <- incoming
	}
}

func (c *StdioClient) failPending(err error) {
	c.pendingMu.Lock()
	if c.transport == nil {
		c.transport = err
	}
	responses := make([]chan incomingMessage, 0, len(c.pending))
	for id, response := range c.pending {
		responses = append(responses, response)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
	for _, response := range responses {
		response <- incomingMessage{err: err}
	}
}

func (c *StdioClient) send(message map[string]any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.closeMu.Lock()
	closed := c.closed
	c.closeMu.Unlock()
	if closed {
		return fmt.Errorf("MCP server %s is closed", c.config.Name)
	}
	if c.stdin == nil {
		return fmt.Errorf("MCP server %s is not running", c.config.Name)
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := c.stdin.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("MCP server %s stdin is closed; stderr: %s", c.config.Name, c.StderrExcerpt(2000))
	}
	return nil
}
