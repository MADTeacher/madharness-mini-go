package mcp

import (
	"fmt"
	"io"
	"os"
	"time"
)

func (c *StdioClient) waitForExit() {
	c.waitErr = c.cmd.Wait()
	close(c.waitDone)
	c.failPending(fmt.Errorf("MCP server %s exited before response; exit_code: %s; stderr: %s", c.config.Name, c.exitCodeText(), c.StderrExcerpt(2000)))
}

func (c *StdioClient) closeStdin() {
	if c.stdin == nil {
		return
	}
	_ = c.stdin.Close()
}

func (c *StdioClient) waitOrSignal() *int {
	select {
	case <-c.waitDone:
		return c.exitCode()
	case <-time.After(2 * time.Second):
	}
	_ = c.cmd.Process.Signal(os.Interrupt)
	select {
	case <-c.waitDone:
		return c.exitCode()
	case <-time.After(2 * time.Second):
	}
	_ = c.cmd.Process.Kill()
	select {
	case <-c.waitDone:
		return c.exitCode()
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (c *StdioClient) closePipes() {
	for _, stream := range []io.Closer{c.stdin, c.stdout, c.stderr} {
		if stream != nil {
			_ = stream.Close()
		}
	}
}

func (c *StdioClient) waitReaders() {
	done := make(chan struct{})
	go func() {
		c.readers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
}

func (c *StdioClient) exitCode() *int {
	if c.cmd == nil || c.cmd.ProcessState == nil {
		return nil
	}
	code := c.cmd.ProcessState.ExitCode()
	return &code
}

func (c *StdioClient) exitCodeText() string {
	if code := c.exitCode(); code != nil {
		return fmt.Sprint(*code)
	}
	return "unknown"
}
