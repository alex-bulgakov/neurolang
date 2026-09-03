package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInitializeAndListTools(t *testing.T) {
	resp, notify := Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`))
	if notify {
		t.Fatal("initialize is not a notification")
	}
	var body map[string]any
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatal(err)
	}
	result := body["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocol: %v", result["protocolVersion"])
	}

	resp, _ = Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatal(err)
	}
	result = body["result"].(map[string]any)
	toolsAny := result["tools"].([]any)
	if len(toolsAny) < 8 {
		t.Fatalf("expected registry tools, got %d", len(toolsAny))
	}
	joined := ""
	for _, t0 := range toolsAny {
		joined += t0.(map[string]any)["name"].(string) + " "
	}
	if !strings.Contains(joined, "fs.read") || !strings.Contains(joined, "http.get") {
		t.Fatalf("missing tools: %s", joined)
	}

	resp, _ = Handle([]byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"time.now"}}`))
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatal(err)
	}
	result = body["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("time.now error: %v", result)
	}
}

func TestUnknownMethod(t *testing.T) {
	resp, _ := Handle([]byte(`{"jsonrpc":"2.0","id":3,"method":"nope"}`))
	var body map[string]any
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatal(err)
	}
	errObj := body["error"].(map[string]any)
	if int(errObj["code"].(float64)) != -32601 {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestInitializedNotification(t *testing.T) {
	_, notify := Handle([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if !notify {
		t.Fatal("expected notification")
	}
}
