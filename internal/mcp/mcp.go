// Package mcp implements a small HTTP JSON-RPC 2.0 surface compatible with the
// Model Context Protocol so agents can inspect and deploy Dockfin resources.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

const ProtocolVersion = "2024-11-05"

// Request is a JSON-RPC 2.0 request. ID is kept raw so notifications
// (no id) and both string/number ids round-trip unchanged.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
)

// Tool describes a callable tool advertised via tools/list.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Backend resolves tool calls against the caller's team. Implementations are
// responsible for team scoping and result bounding.
type Backend interface {
	ListServers(ctx context.Context) (any, error)
	ListProjects(ctx context.Context) (any, error)
	GetApplication(ctx context.Context, id string) (any, error)
	DeployApplication(ctx context.Context, id string, forceRebuild bool) (any, error)
	ListDatabases(ctx context.Context, environmentID string) (any, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

// Tools is the fixed tool catalogue exposed by the server.
func Tools() []Tool {
	return []Tool{
		{
			Name:        "list_servers",
			Description: "List servers registered in the current team.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "list_projects",
			Description: "List projects in the current team.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "get_application",
			Description: "Get a single application by UUID.",
			InputSchema: obj(map[string]any{"id": str("Application UUID")}, "id"),
		},
		{
			Name:        "deploy_application",
			Description: "Queue a deployment for an application.",
			InputSchema: obj(map[string]any{
				"id":            str("Application UUID"),
				"force_rebuild": map[string]any{"type": "boolean", "description": "Rebuild without cache"},
			}, "id"),
		},
		{
			Name:        "list_databases",
			Description: "List databases in the current team, optionally filtered by environment.",
			InputSchema: obj(map[string]any{"environment_id": str("Environment UUID (optional)")}),
		},
	}
}

// Handle dispatches a single JSON-RPC request. It returns nil for
// notifications (requests without an id), which must not be answered.
func Handle(ctx context.Context, b Backend, req Request, serverName, serverVersion string) *Response {
	notification := len(req.ID) == 0
	reply := func(result any, rpcErr *RPCError) *Response {
		if notification {
			return nil
		}
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr}
	}

	switch req.Method {
	case "initialize":
		return reply(map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": serverName, "version": serverVersion},
		}, nil)
	case "ping":
		return reply(map[string]any{}, nil)
	case "notifications/initialized":
		return nil
	case "tools/list":
		return reply(map[string]any{"tools": Tools()}, nil)
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return reply(nil, &RPCError{Code: CodeInvalidParams, Message: "invalid params"})
			}
		}
		result, err := callTool(ctx, b, params.Name, params.Arguments)
		if err != nil {
			return reply(toolContent(err.Error(), true), nil)
		}
		payload, mErr := json.MarshalIndent(result, "", "  ")
		if mErr != nil {
			return reply(nil, &RPCError{Code: CodeInternal, Message: "encode result"})
		}
		return reply(toolContent(string(payload), false), nil)
	default:
		return reply(nil, &RPCError{Code: CodeMethodNotFound, Message: "unknown method " + req.Method})
	}
}

func toolContent(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func callTool(ctx context.Context, b Backend, name string, rawArgs json.RawMessage) (any, error) {
	var args struct {
		ID            string `json:"id"`
		EnvironmentID string `json:"environment_id"`
		ForceRebuild  bool   `json:"force_rebuild"`
	}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %v", err)
		}
	}
	switch name {
	case "list_servers":
		return b.ListServers(ctx)
	case "list_projects":
		return b.ListProjects(ctx)
	case "get_application":
		if args.ID == "" {
			return nil, fmt.Errorf("id is required")
		}
		return b.GetApplication(ctx, args.ID)
	case "deploy_application":
		if args.ID == "" {
			return nil, fmt.Errorf("id is required")
		}
		return b.DeployApplication(ctx, args.ID, args.ForceRebuild)
	case "list_databases":
		return b.ListDatabases(ctx, args.EnvironmentID)
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}
