package lsp

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/compiler/check"
	"github.com/odvcencio/fyx/compiler/diag"
	"github.com/odvcencio/fyx/grammar"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// Server is the Fyx language server.
type Server struct {
	lang *gotreesitter.Language
	docs map[string]string // uri -> content
}

// NewServer creates a new Fyx language server.
func NewServer() *Server {
	lang, err := grammargen.GenerateLanguage(grammar.FyxGrammar())
	if err != nil {
		panic("failed to generate grammar: " + err.Error())
	}
	return &Server{lang: lang, docs: make(map[string]string)}
}

// ValidateDocument parses and validates a single .fyx document, returning diagnostics.
func (s *Server) ValidateDocument(uri string, content string) []diag.Diagnostic {
	file, err := ast.BuildAST(s.lang, []byte(content))
	if err != nil {
		return []diag.Diagnostic{{
			Code:     "F0000",
			Severity: diag.SeverityError,
			Message:  "parse error: " + err.Error(),
			Guide: diag.GuideMessage{
				Summary: "Fyx couldn't understand this file.",
				Explain: "There's a syntax error. Check for missing braces, typos, or incomplete statements.",
			},
		}}
	}
	return check.CheckFile(*file, check.CheckOptions{FilePath: uriToPath(uri)})
}

func uriToPath(uri string) string {
	// Simple file:// prefix strip
	if len(uri) > 7 && uri[:7] == "file://" {
		return uri[7:]
	}
	return uri
}

// HandleMessage processes a JSON-RPC message and returns response messages.
func (s *Server) HandleMessage(raw json.RawMessage) []json.RawMessage {
	var req struct {
		ID     interface{}     `json:"id,omitempty"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID)
	case "initialized":
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(req.Params)
	case "textDocument/didChange":
		return s.handleDidChange(req.Params)
	case "textDocument/didSave":
		return s.handleDidSave(req.Params)
	case "textDocument/completion":
		return s.handleCompletion(req.ID)
	case "textDocument/hover":
		return s.handleHover(req.ID, req.Params)
	case "shutdown":
		return []json.RawMessage{mustJSON(map[string]interface{}{
			"jsonrpc": "2.0", "id": req.ID, "result": nil,
		})}
	case "exit":
		os.Exit(0)
		return nil
	default:
		return nil
	}
}

func (s *Server) handleInitialize(id interface{}) []json.RawMessage {
	result := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync": 1, // Full sync
			"completionProvider": map[string]interface{}{
				"triggerCharacters": []string{" "},
			},
			"hoverProvider": true,
		},
	}
	return []json.RawMessage{mustJSON(map[string]interface{}{
		"jsonrpc": "2.0", "id": id, "result": result,
	})}
}

func (s *Server) handleDidOpen(params json.RawMessage) []json.RawMessage {
	var p struct {
		TextDocument struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	return s.publishDiagnostics(p.TextDocument.URI, p.TextDocument.Text)
}

func (s *Server) handleDidChange(params json.RawMessage) []json.RawMessage {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	if len(p.ContentChanges) == 0 {
		return nil
	}
	text := p.ContentChanges[len(p.ContentChanges)-1].Text
	s.docs[p.TextDocument.URI] = text
	return s.publishDiagnostics(p.TextDocument.URI, text)
}

func (s *Server) handleDidSave(params json.RawMessage) []json.RawMessage {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	if text, ok := s.docs[p.TextDocument.URI]; ok {
		return s.publishDiagnostics(p.TextDocument.URI, text)
	}
	return nil
}

func (s *Server) handleCompletion(id interface{}) []json.RawMessage {
	items := CompletionItems()
	lspItems := make([]map[string]interface{}, len(items))
	for i, item := range items {
		lspItems[i] = map[string]interface{}{
			"label":            item.Label,
			"kind":             item.Kind,
			"detail":           item.Detail,
			"insertText":       item.InsertText,
			"insertTextFormat": 2, // Snippet
		}
	}
	return []json.RawMessage{mustJSON(map[string]interface{}{
		"jsonrpc": "2.0", "id": id, "result": lspItems,
	})}
}

func (s *Server) handleHover(id interface{}, params json.RawMessage) []json.RawMessage {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return []json.RawMessage{mustJSON(map[string]interface{}{
			"jsonrpc": "2.0", "id": id, "result": nil,
		})}
	}

	content, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return []json.RawMessage{mustJSON(map[string]interface{}{
			"jsonrpc": "2.0", "id": id, "result": nil,
		})}
	}

	word := wordAtPosition(content, p.Position.Line, p.Position.Character)
	info := HoverInfo(word)
	if info == "" {
		return []json.RawMessage{mustJSON(map[string]interface{}{
			"jsonrpc": "2.0", "id": id, "result": nil,
		})}
	}

	return []json.RawMessage{mustJSON(map[string]interface{}{
		"jsonrpc": "2.0", "id": id, "result": map[string]interface{}{
			"contents": map[string]string{
				"kind":  "markdown",
				"value": info,
			},
		},
	})}
}

func (s *Server) publishDiagnostics(uri, content string) []json.RawMessage {
	diags := s.ValidateDocument(uri, content)
	lspDiags := make([]map[string]interface{}, len(diags))
	for i, d := range diags {
		severity := 1 // Error
		if d.Severity == diag.SeverityWarning {
			severity = 2
		} else if d.Severity == diag.SeverityNote {
			severity = 3
		}
		line := 0
		if d.HasPrimary() && d.Primary.Start.Line > 0 {
			line = d.Primary.Start.Line - 1 // LSP is 0-based
		}
		lspDiags[i] = map[string]interface{}{
			"range": map[string]interface{}{
				"start": map[string]int{"line": line, "character": 0},
				"end":   map[string]int{"line": line, "character": 1000},
			},
			"severity": severity,
			"code":     d.Code,
			"source":   "fyx",
			"message":  d.Format(true), // Guide-voice by default
		}
	}
	return []json.RawMessage{mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]interface{}{
			"uri":         uri,
			"diagnostics": lspDiags,
		},
	})}
}

func mustJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func wordAtPosition(content string, line, character int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	l := lines[line]
	if character < 0 || character >= len(l) {
		return ""
	}
	// Expand left
	start := character
	for start > 0 && isWordChar(l[start-1]) {
		start--
	}
	// Expand right
	end := character
	for end < len(l) && isWordChar(l[end]) {
		end++
	}
	return l[start:end]
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
