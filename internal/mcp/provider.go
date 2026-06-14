package mcp

import (
	"fmt"
	"regexp"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	tracepkg "github.com/MADTeacher/madharness-mini-go/internal/trace"
)

const toolNameMaxLength = 64

var unsafeToolNameChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// ToolProvider запускает включённые MCP-серверы и отдаёт их tools в registry.
type ToolProvider struct {
	clients []*StdioClient
	spans   map[*StdioClient]*tracepkg.Span
	closed  bool
}

// Specs регистрирует внешние MCP tools рядом со встроенными инструментами.
func (p *ToolProvider) Specs(ctx *tools.Context) ([]tools.Spec, error) {
	configs, err := LoadServerConfigs(ctx.Config, ctx.Policy)
	if err != nil {
		return nil, err
	}
	specs := []tools.Spec{}
	for _, config := range configs {
		span := startTraceSpan(ctx.Trace, "mcp_server", map[string]any{
			"server":  config.Name,
			"command": config.Command,
		})
		traceWriter := ctx.Trace
		if span != nil {
			traceWriter = span.Trace()
		}
		client := NewStdioClient(config)
		listed, err := client.Start()
		if err != nil {
			if traceWriter != nil {
				_ = traceWriter.Write("mcp_server_error", map[string]any{
					"server": config.Name,
					"error":  tools.Clipped(err.Error(), 1000),
				})
			}
			if span != nil {
				span.End("error", map[string]any{"error": tools.Clipped(err.Error(), 1000)})
			}
			client.Close()
			return nil, fmt.Errorf("MCP server %s failed to start: %w", config.Name, err)
		}
		p.clients = append(p.clients, client)
		if span != nil {
			if p.spans == nil {
				p.spans = map[*StdioClient]*tracepkg.Span{}
			}
			p.spans[client] = span
		}
		if traceWriter != nil {
			_ = traceWriter.Write("mcp_server_started", map[string]any{
				"server":      config.Name,
				"command":     config.Command,
				"tools_count": len(listed),
			})
		}
		serverSpecs, err := toolSpecsForServer(config, client, listed)
		if err != nil {
			return nil, err
		}
		specs = append(specs, serverSpecs...)
	}
	return specs, nil
}

// Close останавливает все subprocess-ы MCP provider-а.
func (p *ToolProvider) Close(trace tools.TraceWriter) {
	if p == nil || p.closed {
		return
	}
	p.closed = true
	for _, client := range p.clients {
		span := p.spans[client]
		traceWriter := trace
		if span != nil {
			traceWriter = span.Trace()
		}
		code := client.Close()
		if traceWriter != nil {
			_ = traceWriter.Write("mcp_server_stopped", map[string]any{
				"server":    client.Config().Name,
				"exit_code": code,
			})
		}
		if span != nil {
			span.End("ok", map[string]any{"exit_code": code})
		}
	}
}

func startTraceSpan(trace tools.TraceWriter, name string, fields map[string]any) *tracepkg.Span {
	switch item := trace.(type) {
	case *tracepkg.Trace:
		return item.StartSpan(name, "", fields)
	case *tracepkg.ScopedTrace:
		return item.StartSpan(name, "", fields)
	default:
		return nil
	}
}

func toolSpecsForServer(config ServerConfig, client *StdioClient, listed []map[string]any) ([]tools.Spec, error) {
	specs := []tools.Spec{}
	seen := map[string]bool{}
	for _, item := range listed {
		originalName, ok := item["name"].(string)
		if !ok || originalName == "" {
			return nil, fmt.Errorf("invalid MCP tool from %s: missing name", config.Name)
		}
		exportedName, err := ExportedToolName(config.Name, originalName)
		if err != nil {
			return nil, err
		}
		if seen[exportedName] {
			return nil, fmt.Errorf("duplicate exported MCP tool name: %s", exportedName)
		}
		seen[exportedName] = true
		description, _ := item["description"].(string)
		if description == "" {
			description = fmt.Sprintf("MCP tool %s.%s", config.Name, originalName)
		}
		rawParameters, hasParameters := item["inputSchema"]
		parameters, ok := rawParameters.(map[string]any)
		if !hasParameters {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		} else if !ok {
			return nil, fmt.Errorf("invalid MCP tool %s.%s: inputSchema must be object", config.Name, originalName)
		}
		specs = append(specs, tools.Spec{
			Name:        exportedName,
			Description: fmt.Sprintf("[MCP:%s] %s", config.Name, description),
			Parameters:  parameters,
			Handler:     handler(client, config.Name, originalName, exportedName),
			Effect:      tools.EffectMCP,
		})
	}
	return specs, nil
}

func handler(client *StdioClient, serverName string, toolName string, exportedName string) tools.Handler {
	return func(_ *tools.Context, args map[string]any) tools.Observation {
		result, err := client.CallTool(toolName, args)
		if err != nil {
			return tools.Fail(exportedName, fmt.Sprintf("MCP tool %s.%s failed: %v", serverName, toolName, err))
		}
		return ResultToObservation(exportedName, serverName, toolName, result)
	}
}

// ExportedToolName делает MCP tool name совместимым с OpenAI function name.
func ExportedToolName(serverName string, toolName string) (string, error) {
	safeTool := unsafeToolNameChars.ReplaceAllString(toolName, "_")
	if safeTool == "" {
		return "", fmt.Errorf("invalid MCP tool name: %s", toolName)
	}
	exported := fmt.Sprintf("mcp__%s__%s", serverName, safeTool)
	if len(exported) > toolNameMaxLength {
		return "", fmt.Errorf("MCP tool name is too long for model API: %s", exported)
	}
	return exported, nil
}
