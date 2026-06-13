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
			c.messages <- incomingMessage{err: fmt.Errorf("invalid MCP JSON from stdout: %w: %s", err, clipText(line, 200))}
			continue
		}
		c.messages <- incomingMessage{message: message}
	}
	if err := scanner.Err(); err != nil {
		c.messages <- incomingMessage{err: fmt.Errorf("MCP stdout read error: %w", err)}
	}
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
