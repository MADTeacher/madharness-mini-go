package mcp

import (
	"encoding/json"
	"fmt"
)

// ProtocolVersion фиксирует версию MCP, которую поддерживает учебный клиент.
const ProtocolVersion = "2025-11-25"

// JSONRPCBuilder выдаёт монотонные id и собирает JSON-RPC сообщения.
type JSONRPCBuilder struct {
	nextID int64
}

// NewJSONRPCBuilder создаёт builder с первым request id, равным 1.
func NewJSONRPCBuilder() *JSONRPCBuilder {
	return &JSONRPCBuilder{nextID: 1}
}

// Request собирает request, на который MCP-сервер должен вернуть response.
func (b *JSONRPCBuilder) Request(method string, params map[string]any) map[string]any {
	id := b.nextID
	b.nextID++
	message := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		message["params"] = params
	}
	return message
}

// Notification собирает notification без id.
func (b *JSONRPCBuilder) Notification(method string, params map[string]any) map[string]any {
	message := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		message["params"] = params
	}
	return message
}

// ParseResponse достаёт result из JSON-RPC response или возвращает ошибку протокола.
func ParseResponse(message map[string]any, expectedID int64) (map[string]any, error) {
	if message["jsonrpc"] != "2.0" {
		return nil, fmt.Errorf("invalid JSON-RPC response: missing jsonrpc=2.0")
	}
	if !sameID(message["id"], expectedID) {
		return nil, fmt.Errorf("invalid JSON-RPC response id: expected %d, got %v", expectedID, message["id"])
	}
	if rawError, ok := message["error"]; ok {
		return nil, jsonRPCError(rawError)
	}
	rawResult, ok := message["result"]
	if !ok {
		return nil, fmt.Errorf("invalid JSON-RPC response: missing result")
	}
	result, ok := rawResult.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid JSON-RPC response: result must be object")
	}
	return result, nil
}

func methodNotFoundResponse(requestID any, method string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"error": map[string]any{
			"code":    -32601,
			"message": "Method not found: " + method,
		},
	}
}

func emptyResultResponse(requestID any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result":  map[string]any{},
	}
}

func jsonRPCError(raw any) error {
	if item, ok := raw.(map[string]any); ok {
		code := item["code"]
		text := item["message"]
		return fmt.Errorf("MCP JSON-RPC error %v: %v", code, text)
	}
	return fmt.Errorf("MCP JSON-RPC error: %v", raw)
}

func sameID(raw any, expected int64) bool {
	parsed, ok := idFromAny(raw)
	return ok && parsed == expected
}

func idFromAny(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case float64:
		if value != float64(int64(value)) {
			return 0, false
		}
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
