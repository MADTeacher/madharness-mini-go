package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// ResultToObservation приводит MCP tools/call result к обычному observation.
func ResultToObservation(exportedName string, serverName string, toolName string, result map[string]any) tools.Observation {
	texts := []string{}
	media := []map[string]any{}
	resources := []map[string]any{}
	diagnostics := []string{}
	for _, item := range contentItems(result["content"]) {
		kind, _ := item["type"].(string)
		switch kind {
		case "text":
			texts = append(texts, textContent(item["text"]))
		case "image", "audio":
			media = append(media, mediaMetadata(item, kind))
		case "resource", "resource_link":
			resources = append(resources, resourceMetadata(item))
		default:
			diagnostics = append(diagnostics, fmt.Sprintf("unsupported MCP content type: %v", item["type"]))
		}
	}
	data := map[string]any{}
	if content := strings.TrimSpace(strings.Join(texts, "\n")); content != "" {
		data["content"] = tools.Clipped(content, 0)
	}
	if structured, ok := result["structuredContent"]; ok {
		data["data"] = structured
	}
	if len(media) > 0 {
		data["media"] = media
	}
	if len(resources) > 0 {
		data["resources"] = resources
	}
	if len(diagnostics) > 0 {
		data["diagnostics"] = diagnostics
	}
	summary := fmt.Sprintf("MCP tool %s.%s completed", serverName, toolName)
	if result["isError"] == true {
		if reason, ok := data["content"].(string); ok && reason != "" {
			summary = fmt.Sprintf("MCP tool %s.%s returned error: %s", serverName, toolName, tools.Clipped(reason, 200))
		}
		return tools.Fail(exportedName, summary, data)
	}
	return tools.OK(exportedName, summary, data)
}

func contentItems(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, rawItem := range items {
		if item, ok := rawItem.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func textContent(raw any) string {
	if text, ok := raw.(string); ok {
		return text
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprint(raw)
	}
	return string(encoded)
}

func mediaMetadata(item map[string]any, kind string) map[string]any {
	base64Chars := 0
	if rawData, ok := item["data"].(string); ok {
		base64Chars = len(rawData)
	}
	mimeType, _ := item["mimeType"].(string)
	if mimeType == "" {
		mimeType, _ = item["mime_type"].(string)
	}
	return map[string]any{
		"type":         kind,
		"mime_type":    mimeType,
		"base64_chars": base64Chars,
	}
}

func resourceMetadata(item map[string]any) map[string]any {
	resource, ok := item["resource"].(map[string]any)
	if !ok {
		resource = item
	}
	mimeType, _ := resource["mimeType"].(string)
	if mimeType == "" {
		mimeType, _ = resource["mime_type"].(string)
	}
	name, _ := resource["name"].(string)
	uri, _ := resource["uri"].(string)
	description, _ := resource["description"].(string)
	return map[string]any{
		"uri":         uri,
		"name":        name,
		"mime_type":   mimeType,
		"description": description,
	}
}
