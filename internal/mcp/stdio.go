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

	messages chan incomingMessage
	waitDone chan struct{}
	waitErr  error

	stderrMu  sync.Mutex
	stderrLog []string
	readers   sync.WaitGroup
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
		messages: make(chan incomingMessage, 64),
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
	message := c.rpc.Request(method, params)
	expectedID, _ := message["id"].(int64)
	if err := c.send(message); err != nil {
		return nil, err
	}
	timer := time.NewTimer(c.config.Timeout)
	defer timer.Stop()
	for {
		select {
		case incoming := <-c.messages:
			if incoming.err != nil {
				return nil, incoming.err
			}
			if isServerRequest(incoming.message) {
				methodName, _ := incoming.message["method"].(string)
				if err := c.send(methodNotFoundResponse(incoming.message["id"], methodName)); err != nil {
					return nil, err
				}
				continue
			}
			if _, ok := incoming.message["id"]; !ok {
				continue
			}
			if !sameID(incoming.message["id"], expectedID) {
				return nil, fmt.Errorf("unexpected MCP response id: %v; expected %d", incoming.message["id"], expectedID)
			}
			return ParseResponse(incoming.message, expectedID)
		case <-c.waitDone:
			return nil, fmt.Errorf("MCP server %s exited before response to %s; exit_code: %s; stderr: %s", c.config.Name, method, c.exitCodeText(), c.StderrExcerpt(2000))
		case <-timer.C:
			return nil, fmt.Errorf("MCP request timed out: %s; stderr: %s", method, c.StderrExcerpt(2000))
		}
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

func (c *StdioClient) send(message map[string]any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
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
