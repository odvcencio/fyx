package transpiler

import (
	"strings"
	"testing"
)

func TestEmitterBasic(t *testing.T) {
	e := NewEmitter()
	e.Line("pub struct Foo {")
	e.Indent()
	e.Line("bar: i32,")
	e.Dedent()
	e.Line("}")

	out := e.String()
	if !strings.Contains(out, "pub struct Foo {") {
		t.Errorf("missing struct line: %s", out)
	}
	if !strings.Contains(out, "    bar: i32,") {
		t.Errorf("missing indented field: %s", out)
	}
	if !strings.Contains(out, "\n}") {
		t.Errorf("missing closing brace: %s", out)
	}
}

func TestEmitterBlankLine(t *testing.T) {
	e := NewEmitter()
	e.Line("first")
	e.Blank()
	e.Line("second")

	out := e.String()
	if !strings.Contains(out, "first\n\nsecond") {
		t.Errorf("blank line not emitted correctly: %q", out)
	}
}

func TestEmitterLinef(t *testing.T) {
	e := NewEmitter()
	e.Linef("pub %s: %s,", "speed", "f32")

	out := e.String()
	if !strings.Contains(out, "pub speed: f32,") {
		t.Errorf("Linef not formatting correctly: %s", out)
	}
}

func TestEmitterDedentFloor(t *testing.T) {
	e := NewEmitter()
	e.Dedent() // should not go negative
	e.Dedent()
	e.Line("no indent")

	out := e.String()
	if out != "no indent\n" {
		t.Errorf("unexpected output after extra dedent: %q", out)
	}
}

func TestEmitterNestedIndent(t *testing.T) {
	e := NewEmitter()
	e.Line("level0")
	e.Indent()
	e.Line("level1")
	e.Indent()
	e.Line("level2")
	e.Dedent()
	e.Line("level1again")
	e.Dedent()
	e.Line("level0again")

	out := e.String()
	lines := strings.Split(out, "\n")
	expected := []string{
		"level0",
		"    level1",
		"        level2",
		"    level1again",
		"level0again",
	}
	for i, exp := range expected {
		if i >= len(lines) {
			t.Fatalf("missing line %d: expected %q", i, exp)
		}
		if lines[i] != exp {
			t.Errorf("line %d: got %q, want %q", i, lines[i], exp)
		}
	}
}
