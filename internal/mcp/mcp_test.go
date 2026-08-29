package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type fakeBackend struct{ deployed string }

func (f *fakeBackend) ListServers(context.Context) (any, error) {
	return map[string]any{"servers": []any{}}, nil
}
func (f *fakeBackend) ListProjects(context.Context) (any, error) {
	return map[string]any{"projects": []any{}}, nil
}
func (f *fakeBackend) ListApplications(context.Context) (any, error) {
	return map[string]any{"applications": []any{}}, nil
}
func (f *fakeBackend) GetApplication(_ context.Context, id string) (any, error) {
	if id != "app-1" {
		return nil, fmt.Errorf("not found")
	}
	return map[string]any{"id": id}, nil
}
func (f *fakeBackend) DeployApplication(_ context.Context, id string, force bool) (any, error) {
	f.deployed = id
	return map[string]any{"status": "queued", "force": force}, nil
}
func (f *fakeBackend) StopApplication(_ context.Context, id string) (any, error) {
	return map[string]any{"id": id, "status": "exited"}, nil
}
func (f *fakeBackend) ListDatabases(context.Context, string) (any, error) {
	return map[string]any{"databases": []any{}}, nil
}
func (f *fakeBackend) ListServices(context.Context) (any, error) {
	return map[string]any{"services": []any{}}, nil
}
func (f *fakeBackend) DeployService(_ context.Context, id string) (any, error) {
	return map[string]any{"status": "queued", "id": id}, nil
}

func handle(t *testing.T, b Backend, method, params string) *Response {
	t.Helper()
	req := Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: method}
	if params != "" {
		req.Params = json.RawMessage(params)
	}
	return Handle(context.Background(), b, req, "dockfin", "test")
}

func TestToolsList(t *testing.T) {
	resp := handle(t, &fakeBackend{}, "tools/list", "")
	if resp == nil || resp.Error != nil {
		t.Fatalf("unexpected response %+v", resp)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]Tool)
	if len(tools) != 9 {
		t.Fatalf("expected 9 tools, got %d", len(tools))
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	req := Request{JSONRPC: "2.0", Method: "tools/list"}
	if resp := Handle(context.Background(), &fakeBackend{}, req, "dockfin", "test"); resp != nil {
		t.Fatalf("expected nil response for notification, got %+v", resp)
	}
}

func TestToolCallDeploy(t *testing.T) {
	b := &fakeBackend{}
	resp := handle(t, b, "tools/call", `{"name":"deploy_application","arguments":{"id":"app-1","force_rebuild":true}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("unexpected response %+v", resp)
	}
	if b.deployed != "app-1" {
		t.Fatalf("deploy not called, got %q", b.deployed)
	}
	if isErr := resp.Result.(map[string]any)["isError"].(bool); isErr {
		t.Fatal("expected non-error tool result")
	}
}

func TestToolCallErrorsAreToolErrors(t *testing.T) {
	resp := handle(t, &fakeBackend{}, "tools/call", `{"name":"get_application","arguments":{}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected JSON-RPC success with tool error, got %+v", resp)
	}
	result := resp.Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatal("expected isError=true")
	}
	content := result["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "id is required") {
		t.Fatalf("unexpected message %v", content[0]["text"])
	}
}

func TestUnknownMethod(t *testing.T) {
	resp := handle(t, &fakeBackend{}, "does/not/exist", "")
	if resp == nil || resp.Error == nil || resp.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected method-not-found, got %+v", resp)
	}
}
