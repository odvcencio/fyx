package lsp

import (
	"encoding/json"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestServer_ValidateDocument_DuplicateField(t *testing.T) {
	s := NewServer()
	src := "script Player {\n    inspect speed: f32 = 5.0\n    inspect speed: f32 = 10.0\n}"
	diags := s.ValidateDocument("file:///test/player.fyx", src)
	if len(diags) == 0 {
		t.Error("expected diagnostics for duplicate field, got none")
	}
}

func TestServer_ValidateDocument_EmptyScript(t *testing.T) {
	s := NewServer()
	diags := s.ValidateDocument("file:///test/empty.fyx", "script Empty {\n}")
	if len(diags) == 0 {
		t.Error("expected diagnostics for empty script, got none")
	}
}

func TestServer_ValidateDocument_ValidFile(t *testing.T) {
	s := NewServer()
	diags := s.ValidateDocument("file:///test/player.fyx", "script Player {\n    inspect speed: f32 = 5.0\n    on update(ctx) {\n        self.speed += 1.0;\n    }\n}")
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for valid file, got %d", len(diags))
	}
}

func TestHandleMessage_Initialize(t *testing.T) {
	s := NewServer()
	msg := mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	responses := s.HandleMessage(msg)
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(responses[0], &resp); err != nil {
		t.Fatal(err)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result in response")
	}
	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities in result")
	}
	if caps["hoverProvider"] != true {
		t.Error("expected hoverProvider to be true")
	}
}

func TestHandleMessage_DidOpen_PublishesDiagnostics(t *testing.T) {
	s := NewServer()
	msg := mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":  "file:///test/bad.fyx",
				"text": "script Player {\n    inspect speed: f32 = 5.0\n    inspect speed: f32 = 10.0\n}",
			},
		},
	})
	responses := s.HandleMessage(msg)
	if len(responses) == 0 {
		t.Fatal("expected diagnostic notification")
	}
	var notif map[string]interface{}
	if err := json.Unmarshal(responses[0], &notif); err != nil {
		t.Fatal(err)
	}
	if notif["method"] != "textDocument/publishDiagnostics" {
		t.Errorf("expected publishDiagnostics, got %v", notif["method"])
	}
}

func TestHandleMessage_Shutdown(t *testing.T) {
	s := NewServer()
	msg := mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      99,
		"method":  "shutdown",
		"params":  nil,
	})
	responses := s.HandleMessage(msg)
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
}

func TestUriToPath(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///home/user/test.fyx", "/home/user/test.fyx"},
		{"test.fyx", "test.fyx"},
	}
	for _, tt := range tests {
		got := uriToPath(tt.uri)
		if got != tt.want {
			t.Errorf("uriToPath(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestWordAtPosition(t *testing.T) {
	content := "script Player {\n    inspect speed: f32\n}"
	tests := []struct {
		line, char int
		want       string
	}{
		{0, 0, "script"},
		{0, 3, "script"},
		{0, 7, "Player"},
		{1, 6, "inspect"},
		{1, 16, "speed"},
	}
	for _, tt := range tests {
		got := wordAtPosition(content, tt.line, tt.char)
		if got != tt.want {
			t.Errorf("wordAtPosition(line=%d, char=%d) = %q, want %q", tt.line, tt.char, got, tt.want)
		}
	}
}
