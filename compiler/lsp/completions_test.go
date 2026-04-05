package lsp

import (
	"strings"
	"testing"
)

func TestCompletionItems_ContainsKeywords(t *testing.T) {
	items := CompletionItems()
	if len(items) == 0 {
		t.Fatal("no completion items")
	}
	found := map[string]bool{}
	for _, item := range items {
		found[item.Label] = true
	}
	required := []string{
		"script", "inspect", "node", "reactive", "derived",
		"signal", "on update", "on start", "component", "system",
		"state", "timer", "emit", "connect", "watch",
	}
	for _, kw := range required {
		if !found[kw] {
			t.Errorf("missing completion for %q", kw)
		}
	}
}

func TestCompletionItems_HaveSnippets(t *testing.T) {
	items := CompletionItems()
	for _, item := range items {
		if item.InsertText == "" {
			t.Errorf("completion %q has empty InsertText", item.Label)
		}
		if item.Detail == "" {
			t.Errorf("completion %q has empty Detail", item.Label)
		}
	}
}

func TestHoverInfo_InspectKeyword(t *testing.T) {
	info := HoverInfo("inspect")
	if info == "" {
		t.Error("no hover info for 'inspect'")
	}
	if !strings.Contains(info, "editor") {
		t.Error("hover for 'inspect' should mention the editor")
	}
}

func TestHoverInfo_ScriptKeyword(t *testing.T) {
	info := HoverInfo("script")
	if info == "" {
		t.Error("no hover info for 'script'")
	}
	if !strings.Contains(info, "script") {
		t.Error("hover for 'script' should describe scripts")
	}
}

func TestHoverInfo_UnknownKeyword(t *testing.T) {
	info := HoverInfo("xyznotakeyword")
	if info != "" {
		t.Error("expected empty hover for unknown keyword")
	}
}

func TestHoverInfo_AllCompletionsHaveHover(t *testing.T) {
	// Every single-word completion label should have hover documentation.
	items := CompletionItems()
	for _, item := range items {
		// Multi-word labels like "on update" won't match a single word hover
		if strings.Contains(item.Label, " ") {
			continue
		}
		info := HoverInfo(item.Label)
		if info == "" {
			t.Errorf("completion %q has no matching hover documentation", item.Label)
		}
	}
}
