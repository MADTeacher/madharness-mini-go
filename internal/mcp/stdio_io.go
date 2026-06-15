package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *StdioClient) startReaders() {
	c.readers.Add(2)
	go c.readStdout()
	go c.readStderr()
}

func (c *StdioClient) readStdout() {
	defer c.readers.Done()
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		message := map[string]any{}
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			c.failPending(fmt.Errorf("invalid MCP JSON from stdout: %w: %s", err, clipText(line, 200)))
			return
		}
		c.dispatchStdoutMessage(message)
	}
	if err := scanner.Err(); err != nil {
		c.failPending(fmt.Errorf("MCP stdout read error: %w", err))
	}
}

func (c *StdioClient) dispatchStdoutMessage(message map[string]any) {
	if isServerRequest(message) {
		methodName, _ := message["method"].(string)
		response := methodNotFoundResponse(message["id"], methodName)
		if methodName == "ping" {
			response = emptyResultResponse(message["id"])
		}
		if err := c.send(response); err != nil {
			c.failPending(err)
		}
		return
	}
	rawID, ok := message["id"]
	if !ok {
		return
	}
	id, ok := idFromAny(rawID)
	if !ok {
		return
	}
	c.completePending(id, incomingMessage{message: message})
}

func (c *StdioClient) readStderr() {
	defer c.readers.Done()
	scanner := bufio.NewScanner(c.stderr)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		c.stderrMu.Lock()
		c.stderrLog = append(c.stderrLog, scanner.Text()+"\n")
		if len(c.stderrLog) > 20 {
			c.stderrLog = c.stderrLog[len(c.stderrLog)-20:]
		}
		c.stderrMu.Unlock()
	}
}

// StderrExcerpt возвращает короткий stderr для диагностики запуска.
func (c *StdioClient) StderrExcerpt(limit int) string {
	c.stderrMu.Lock()
	defer c.stderrMu.Unlock()
	return strings.TrimSpace(clipText(strings.Join(c.stderrLog, ""), limit))
}
