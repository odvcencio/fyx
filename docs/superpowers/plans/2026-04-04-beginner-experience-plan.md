# Fyx Beginner Experience — Vertical Slice

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make it possible for someone who knows basic programming (but not Rust) to open VS Code, write `.fyx`, get helpful error messages, and follow a tutorial to get a game object on screen.

**Architecture:** A diagnostic engine validates AST before transpilation, catching the 15 most common beginner mistakes with guide-voice explanations. An LSP server (`fyxc lsp`) wraps the diagnostic engine and serves keyword completions + hover docs over stdio. The VS Code extension upgrades from syntax-only to full language client.

**Tech Stack:** Go (LSP server, diagnostic engine), TypeScript (VS Code extension), Markdown (tutorial docs). No new dependencies beyond `go.lsp.dev/protocol` or equivalent for LSP JSON-RPC.

---

## Diagnostic Rule Index

| Code | Description | Task | Severity |
|------|-------------|------|----------|
| F0000 | Parse error (syntax) | 8 | error |
| F0002 | Missing field type | 3 | error |
| F0005 | Signal not found (bad connect) | 5 | error |
| F0006 | Emit arg count mismatch | 5 | error |
| F0007 | Duplicate field name | 3 | error |
| F0008 | Duplicate script name | 3 | error |
| F0010 | `go` to unknown state | 4 | error |
| F0011 | `watch` on non-reactive field | 6 | error |
| F0012 | `derived` without reactive dependency | 6 | warning |
| F0013 | Node path without quotes | 3 | error |
| F0014 | `dt` used outside `on update` | 4 | error |
| F0015 | Empty script | 6 | warning |

Rules 1 (unknown field via `self.x`), 3 (handler outside script), 4 (unknown handler name), and 9 (invalid field modifier) are caught at parse time by tree-sitter and are not re-implemented in the diagnostic engine. They may be added later if finer error messages are needed.

---

## File Structure

### New Files

```
compiler/check/                          # Semantic validation (diagnostic engine)
  check.go                               # CheckFile(ast.File, CheckOptions) -> []diag.Diagnostic
  check_test.go                          # Tests for all 15 diagnostic rules
  rules.go                               # Individual rule functions
  guide.go                               # Guide-voice message templates

compiler/lsp/                            # LSP server core
  server.go                              # LSP handler: initialize, didOpen, didChange, completion, hover
  server_test.go                         # Tests for LSP message handling
  completions.go                         # Keyword/snippet completion catalog
  hover.go                               # Hover info catalog for Fyx keywords

cmd/fyxc/lsp.go                         # `fyxc lsp` subcommand entry point

editors/vscode/src/extension.ts          # VS Code language client activation
editors/vscode/package.json              # Updated with LSP client config
editors/vscode/tsconfig.json             # TypeScript config for extension
editors/vscode/.vscodeignore             # Extension packaging ignore

docs/tutorial/                           # "Your First Fyx Game" tutorial
  01-setup.md
  02-first-script.md
  03-make-it-move.md
  04-editor-fields.md
  05-input.md
  06-state-machines.md
  07-signals.md
  08-whats-next.md

testdata/check/                          # Diagnostic engine test fixtures
  unknown_field.fyx
  missing_type.fyx
  handler_outside_script.fyx
  ... (one per rule)
```

### Modified Files

```
cmd/fyxc/main.go                        # Add "lsp" subcommand dispatch
compiler/diag/diagnostic.go             # Add GuideMessage field, Format() method
compiler/span/span.go                   # No changes expected
```

---

## Task 1: Extend Diagnostic Type with Guide-Voice Support

**Files:**
- Modify: `compiler/diag/diagnostic.go`
- Test: `compiler/diag/diagnostic_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// compiler/diag/diagnostic_test.go
package diag

import (
	"strings"
	"testing"
)

func TestDiagnosticFormat_Verbose(t *testing.T) {
	d := Diagnostic{
		Code:     "F0002",
		Severity: SeverityError,
		Message:  "field `speed` has no type",
		Guide: GuideMessage{
			Summary: "`speed` needs a type.",
			Explain: "Every field must declare what kind of value it holds.",
			Suggest: `Try:

    inspect speed: f32 = 0.0

"f32" means a decimal number.`,
		},
	}

	verbose := d.Format(true)
	if verbose == "" {
		t.Fatal("verbose format returned empty")
	}
	if !strings.Contains(verbose, "needs a type") {
		t.Errorf("verbose should contain guide summary, got:\n%s", verbose)
	}

	terse := d.Format(false)
	if !strings.Contains(terse, "F0002") {
		t.Errorf("terse should contain error code, got:\n%s", terse)
	}
	if strings.Contains(terse, "needs a type") {
		t.Errorf("terse should NOT contain guide text, got:\n%s", terse)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/diag/ -v -run TestDiagnosticFormat`
Expected: FAIL — `GuideMessage` type and `Format` method don't exist.

- [ ] **Step 3: Implement GuideMessage and Format**

Add to `compiler/diag/diagnostic.go`:

```go
// GuideMessage provides beginner-friendly explanation of a diagnostic.
type GuideMessage struct {
	Summary string // One-line plain-language description
	Explain string // Why this happened
	Suggest string // What to do about it, with code example
}

// Add Guide field to Diagnostic struct:
// Guide GuideMessage

// Format renders the diagnostic as a human-readable string.
// When verbose is true, uses guide-voice. When false, uses compiler-voice.
func (d Diagnostic) Format(verbose bool) string {
	if verbose && d.Guide.Summary != "" {
		var b strings.Builder
		if d.HasPrimary() {
			fmt.Fprintf(&b, "%s:%d — %s\n", d.Primary.File, d.Primary.Start.Line, d.Guide.Summary)
		} else {
			fmt.Fprintf(&b, "%s\n", d.Guide.Summary)
		}
		if d.Guide.Explain != "" {
			fmt.Fprintf(&b, "\n%s\n", d.Guide.Explain)
		}
		if d.Guide.Suggest != "" {
			fmt.Fprintf(&b, "\n%s\n", d.Guide.Suggest)
		}
		return b.String()
	}

	// Terse mode
	var b strings.Builder
	if d.HasPrimary() {
		fmt.Fprintf(&b, "%s[%s]: %s\n  --> %s:%d:%d",
			d.Severity, d.Code, d.Message,
			d.Primary.File, d.Primary.Start.Line, d.Primary.Start.Column)
	} else {
		fmt.Fprintf(&b, "%s[%s]: %s", d.Severity, d.Code, d.Message)
	}
	return b.String()
}
```

Add `"fmt"` and `"strings"` to imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/diag/ -v -run TestDiagnosticFormat`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add compiler/diag/diagnostic.go compiler/diag/diagnostic_test.go
git commit -m "add guide-voice support to diagnostic type"
```

---

## Task 2: Diagnostic Engine — Core Framework

**Files:**
- Create: `compiler/check/check.go`
- Create: `compiler/check/check_test.go`

- [ ] **Step 1: Write the failing test**

```go
// compiler/check/check_test.go
package check

import (
	"testing"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/compiler/diag"
)

func TestCheckFile_EmptyFile_NoDiagnostics(t *testing.T) {
	file := ast.File{}
	diags := CheckFile(file, CheckOptions{})
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty file, got %d", len(diags))
	}
}

func TestCheckFile_ValidScript_NoDiagnostics(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Default: "5.0", Line: 2},
			},
			Handlers: []ast.Handler{
				{Kind: ast.HandlerUpdate, Body: "self.speed += 1.0;", Line: 3, BodyLine: 3},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{})
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for valid script, got %d: %v", len(diags), diags)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run TestCheckFile`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement CheckFile framework**

```go
// compiler/check/check.go
package check

import (
	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/compiler/diag"
	"github.com/odvcencio/fyx/compiler/span"
)

// SignalIndex maps "ScriptName::signalName" to declared parameter lists.
// This is a local type alias to avoid importing transpiler (wrong dependency direction).
// The caller (cmd/fyxc) converts transpiler.SignalIndex to this type — they are both
// map[string][]ast.Param so the conversion is free.
type SignalIndex = map[string][]ast.Param

// CheckOptions configures semantic validation.
type CheckOptions struct {
	FilePath    string
	SignalIndex SignalIndex
}

// CheckFile validates an AST and returns diagnostics for beginner mistakes.
func CheckFile(file ast.File, opts CheckOptions) []diag.Diagnostic {
	ctx := &checkCtx{
		file:     file,
		opts:     opts,
		filePath: span.FileID(opts.FilePath),
	}
	ctx.checkScripts()
	ctx.checkComponents()
	ctx.checkSystems()
	return ctx.diags
}

type checkCtx struct {
	file     ast.File
	opts     CheckOptions
	filePath span.FileID
	diags    []diag.Diagnostic
}

func (c *checkCtx) add(d diag.Diagnostic) {
	c.diags = append(c.diags, d)
}

func (c *checkCtx) span(line int) span.Span {
	return span.Span{
		File:  c.filePath,
		Start: span.Point{Line: line, Column: 1},
	}
}

func (c *checkCtx) checkScripts() {
	seen := map[string]int{}
	for _, script := range c.file.Scripts {
		if prev, ok := seen[script.Name]; ok {
			c.add(duplicateScript(script.Name, c.span(script.Line), prev))
		}
		seen[script.Name] = script.Line
		c.checkScript(script)
	}
}

func (c *checkCtx) checkScript(script ast.Script) {
	c.checkDuplicateFields(script)
	c.checkHandlers(script)
	c.checkStates(script)
	c.checkConnects(script)
	c.checkWatches(script)
	c.checkEmitArgCounts(script)
	c.checkEmpty(script)
}

// Stubs — implemented in rules.go (Tasks 3-6). Present here so Task 2 compiles.
func (c *checkCtx) checkDuplicateFields(_ ast.Script) {}
func (c *checkCtx) checkHandlers(_ ast.Script)        {}
func (c *checkCtx) checkStates(_ ast.Script)           {}
func (c *checkCtx) checkConnects(_ ast.Script)         {}
func (c *checkCtx) checkWatches(_ ast.Script)          {}
func (c *checkCtx) checkEmitArgCounts(_ ast.Script)    {}
func (c *checkCtx) checkEmpty(_ ast.Script)            {}
func (c *checkCtx) checkComponents()                    {}
func (c *checkCtx) checkSystems()                       {}
```

**Note:** The stub methods are replaced by real implementations in Tasks 3-6. Each task should delete the corresponding stub when adding the real function to `rules.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run TestCheckFile`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add compiler/check/check.go compiler/check/check_test.go
git commit -m "add semantic check framework for AST validation"
```

---

## Task 3: Diagnostic Rules — Field Validation (Rules 1-4, 7, 9)

**Files:**
- Create: `compiler/check/rules.go`
- Create: `compiler/check/guide.go`
- Modify: `compiler/check/check.go` (wire rules)
- Modify: `compiler/check/check_test.go` (add tests)

These rules cover the most common field-related beginner mistakes.

- [ ] **Step 1: Write failing tests for field rules**

Add to `compiler/check/check_test.go`:

```go
func TestRule_DuplicateFieldName(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Line: 2},
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Line: 3},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0007")
}

func TestRule_MissingFieldType(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "", Line: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0002")
}

func TestRule_InvalidFieldModifier(t *testing.T) {
	// This is caught at parse time, but we test the diagnostic message exists
	// by calling the rule function directly.
}

func TestRule_NodePathWithoutQuotes(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldNode, Name: "muzzle", TypeExpr: "Node", Default: "MuzzlePoint", Line: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0013")
}

func TestRule_DuplicateScript(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{Name: "Player", Line: 1},
			{Name: "Player", Line: 5},
		},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0008")
}

func assertHasCode(t *testing.T, diags []diag.Diagnostic, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			return
		}
	}
	codes := make([]string, len(diags))
	for i, d := range diags {
		codes[i] = d.Code
	}
	t.Errorf("expected diagnostic code %s, got codes: %v", code, codes)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run TestRule_`
Expected: FAIL — rules not implemented.

- [ ] **Step 3: Implement guide message templates**

```go
// compiler/check/guide.go
package check

import (
	"fmt"

	"github.com/odvcencio/fyx/compiler/diag"
	"github.com/odvcencio/fyx/compiler/span"
)

func missingFieldType(fieldName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0002",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("field `%s` has no type", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("`%s` needs a type.", fieldName),
			Explain: "Every field must declare what kind of value it holds.",
			Suggest: fmt.Sprintf(`Try:

    inspect %s: f32 = 0.0

"f32" means a decimal number. Other common types:
  i32    — whole number
  bool   — true or false
  String — text
  Vector3 — 3D position (x, y, z)`, fieldName),
		},
	}
}

func duplicateField(fieldName, scriptName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0007",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("duplicate field `%s` in script `%s`", fieldName, scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("There's already a field called `%s` in this script.", fieldName),
			Explain: "Each field name can only appear once per script.",
			Suggest: "Rename one of them or remove the duplicate.",
		},
	}
}

func duplicateScript(scriptName string, s span.Span, prevLine int) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0008",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("duplicate script `%s`", scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("Another script is already named `%s` (line %d).", scriptName, prevLine),
			Explain: "Script names must be unique within a file.",
			Suggest: "Rename one of them.",
		},
	}
}

func nodePathWithoutQuotes(fieldName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0013",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("node path for `%s` must be quoted", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("Node paths need quotes around them."),
			Explain: "The path tells Fyx where to find this node in your scene tree.",
			Suggest: fmt.Sprintf(`Try:

    node %s: Node = "MuzzlePoint"

The path uses "/" to go deeper: "Turret/Muzzle"`, fieldName),
		},
	}
}
```

- [ ] **Step 4: Implement field validation rules**

```go
// compiler/check/rules.go
package check

import (
	"strings"

	"github.com/odvcencio/fyx/ast"
)

func (c *checkCtx) checkDuplicateFields(script ast.Script) {
	seen := map[string]int{}
	for _, field := range script.Fields {
		if prev, ok := seen[field.Name]; ok {
			c.add(duplicateField(field.Name, script.Name, c.span(field.Line)))
			_ = prev
		}
		seen[field.Name] = field.Line

		if field.TypeExpr == "" && field.Modifier != ast.FieldDerived {
			c.add(missingFieldType(field.Name, c.span(field.Line)))
		}

		if (field.Modifier == ast.FieldNode || field.Modifier == ast.FieldNodes) && field.Default != "" {
			if !strings.HasPrefix(field.Default, "\"") {
				c.add(nodePathWithoutQuotes(field.Name, c.span(field.Line)))
			}
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run TestRule_`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add compiler/check/rules.go compiler/check/guide.go compiler/check/check.go compiler/check/check_test.go
git commit -m "add field validation rules with guide-voice messages"
```

---

## Task 4: Diagnostic Rules — Handler and State Validation (Rules 3-4, 10, 14)

**Files:**
- Modify: `compiler/check/rules.go`
- Modify: `compiler/check/guide.go`
- Modify: `compiler/check/check_test.go`

- [ ] **Step 1: Write failing tests**

Note: Rules 3 (handler outside script) and 4 (unknown handler name) are caught at parse time by tree-sitter and do not need diagnostic rules. This task implements `go` validation (F0010) and `dt` validation (F0014).

```go
func TestRule_GoToUnknownState(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Enemy",
			States: []ast.State{
				{Name: "idle", Line: 2, Handlers: []ast.StateHandler{
					{Kind: ast.StateHandlerUpdate, Body: "go flying;", Line: 3, BodyLine: 3},
				}},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "enemy.fyx"})
	assertHasCode(t, diags, "F0010")
}

func TestRule_DtOutsideUpdate(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Handlers: []ast.Handler{
				{Kind: ast.HandlerInit, Body: "let x = dt;", Line: 2, BodyLine: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0014")
}

func TestRule_DtInUpdate_NoDiagnostic(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Handlers: []ast.Handler{
				{Kind: ast.HandlerUpdate, Body: "let x = dt;", Line: 2, BodyLine: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{})
	assertNoCode(t, diags, "F0014")
}

func assertNoCode(t *testing.T, diags []diag.Diagnostic, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			t.Errorf("did NOT expect diagnostic code %s but found: %s", code, d.Message)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run "TestRule_(Go|Dt)"`
Expected: FAIL — rules not implemented.

- [ ] **Step 3: Add guide messages for handler/state rules**

Add to `compiler/check/guide.go` (add `"strings"` to the import block):

```go
func goToUnknownState(targetState, scriptName string, knownStates []string, s span.Span) diag.Diagnostic {
	stateList := "none defined"
	if len(knownStates) > 0 {
		stateList = strings.Join(knownStates, ", ")
	}
	return diag.Diagnostic{
		Code:     "F0010",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("`go %s` but no state `%s` exists in `%s`", targetState, targetState, scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("`go %s` — but there's no state called `%s` in this script.", targetState, targetState),
			Explain: "The `go` keyword switches to another state. That state must be declared in the same script.",
			Suggest: fmt.Sprintf(`Available states: %s

To add the missing state:

    state %s {
        on update { }
    }`, stateList, targetState),
		},
	}
}

func dtOutsideUpdate(handlerName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0014",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("`dt` used outside `on update`"),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "`dt` (delta time) is only available inside `on update`.",
			Explain: "Delta time is the seconds since the last frame. It only makes sense in code that runs every frame.",
			Suggest: `Move this code into an "on update" handler, or pass dt as a parameter from update.`,
		},
	}
}
```

- [ ] **Step 4: Implement handler/state validation rules**

Add to `compiler/check/rules.go` (add `"regexp"` to the import block):

```go
var goStateRe = regexp.MustCompile(`\bgo\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)
var bareDtRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])dt\b`)

func (c *checkCtx) checkHandlers(script ast.Script) {
	for _, h := range script.Handlers {
		if h.Kind != ast.HandlerUpdate && bareDtRe.MatchString(h.Body) {
			c.add(dtOutsideUpdate(handlerKindName(h.Kind), c.span(h.BodyLine)))
		}
	}
}

func (c *checkCtx) checkStates(script ast.Script) {
	stateNames := make([]string, len(script.States))
	stateSet := make(map[string]bool, len(script.States))
	for i, state := range script.States {
		stateNames[i] = state.Name
		stateSet[state.Name] = true
	}

	for _, state := range script.States {
		for _, handler := range state.Handlers {
			for _, match := range goStateRe.FindAllStringSubmatch(handler.Body, -1) {
				target := match[1]
				if !stateSet[target] {
					c.add(goToUnknownState(target, script.Name, stateNames, c.span(handler.BodyLine)))
				}
			}
		}
	}
}

func handlerKindName(k ast.HandlerKind) string {
	switch k {
	case ast.HandlerInit:
		return "init"
	case ast.HandlerStart:
		return "start"
	case ast.HandlerUpdate:
		return "update"
	case ast.HandlerDeinit:
		return "deinit"
	case ast.HandlerEvent:
		return "event"
	case ast.HandlerMessage:
		return "message"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add compiler/check/
git commit -m "add handler and state validation rules"
```

---

## Task 5: Diagnostic Rules — Signal Validation (Rules 5-6)

**Files:**
- Modify: `compiler/check/rules.go`
- Modify: `compiler/check/guide.go`
- Modify: `compiler/check/check_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRule_ConnectSignalNotFound(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "ScoreTracker",
			Connects: []ast.Connect{
				{ScriptName: "Enemy", SignalName: "died", Line: 2, BodyLine: 3, Body: "self.score += 1;"},
			},
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "score", TypeExpr: "i32", Line: 1},
			},
		}},
	}
	// No signal index entry for Enemy::died
	diags := CheckFile(file, CheckOptions{FilePath: "score.fyx", SignalIndex: SignalIndex{}})
	assertHasCode(t, diags, "F0005")
}

func TestRule_ConnectSignalFound_NoDiagnostic(t *testing.T) {
	idx := SignalIndex{
		"Enemy::died": []ast.Param{{Name: "position", TypeExpr: "Vector3"}},
	}
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "ScoreTracker",
			Connects: []ast.Connect{
				{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Line: 2, BodyLine: 3, Body: "self.score += 1;"},
			},
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "score", TypeExpr: "i32", Line: 1},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{SignalIndex: idx})
	assertNoCode(t, diags, "F0005")
}

func TestRule_EmitArgCountMismatch(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Weapon",
			Signals: []ast.Signal{
				{Name: "fired", Params: []ast.Param{
					{Name: "position", TypeExpr: "Vector3"},
					{Name: "direction", TypeExpr: "Vector3"},
				}, Line: 1},
			},
			Handlers: []ast.Handler{
				{Kind: ast.HandlerUpdate, Body: "emit fired(self.position());", Line: 3, BodyLine: 3},
			},
		}},
	}
	// Build a local signal index from this file's declarations
	idx := SignalIndex{}
	for _, script := range file.Scripts {
		for _, sig := range script.Signals {
			idx[script.Name+"::"+sig.Name] = sig.Params
		}
	}
	diags := CheckFile(file, CheckOptions{SignalIndex: idx})
	assertHasCode(t, diags, "F0006")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run "TestRule_(Connect|Emit)"`
Expected: FAIL

- [ ] **Step 3: Add guide messages**

Add to `compiler/check/guide.go`:

```go
func signalNotFound(scriptName, signalName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0005",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("no signal `%s::%s` found", scriptName, signalName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("No signal `%s::%s` found.", scriptName, signalName),
			Explain: "A `connect` block listens for a signal from another script. That signal must be declared.",
			Suggest: fmt.Sprintf(`Check that:
  1. The script is named exactly "%s" (case matters)
  2. It declares: signal %s(...)
  3. The file containing "%s" is in the same project`, scriptName, signalName, scriptName),
		},
	}
}

func emitArgCountMismatch(signalName string, expected, got int, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0006",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("`%s` expects %d args, got %d", signalName, expected, got),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("`%s` expects %d values but got %d.", signalName, expected, got),
			Explain: "When you emit a signal, you must provide all the values it was declared with.",
			Suggest: fmt.Sprintf("Check the signal declaration and make sure you're passing all %d arguments.", expected),
		},
	}
}
```

- [ ] **Step 4: Implement signal validation rules**

Add to `compiler/check/rules.go`:

```go
var emitPrefixCheckRe = regexp.MustCompile(`emit\s+([A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)?)\(`)

func (c *checkCtx) checkConnects(script ast.Script) {
	if len(c.opts.SignalIndex) == 0 {
		return
	}
	for _, conn := range script.Connects {
		key := conn.ScriptName + "::" + conn.SignalName
		if _, ok := c.opts.SignalIndex[key]; !ok {
			c.add(signalNotFound(conn.ScriptName, conn.SignalName, c.span(conn.Line)))
		}
	}
}

func (c *checkCtx) checkEmitArgCounts(script ast.Script) {
	idx := c.opts.SignalIndex
	if len(idx) == 0 {
		// Build local-only index from this script's signals
		idx = make(transpiler.SignalIndex)
		for _, sig := range script.Signals {
			idx[script.Name+"::"+sig.Name] = sig.Params
		}
	}

	for _, h := range script.Handlers {
		c.checkEmitInBody(h.Body, script.Name, idx, h.BodyLine)
	}
	for _, state := range script.States {
		for _, sh := range state.Handlers {
			c.checkEmitInBody(sh.Body, script.Name, idx, sh.BodyLine)
		}
	}
}

func (c *checkCtx) checkEmitInBody(body, scriptName string, idx transpiler.SignalIndex, baseLine int) {
	for _, match := range emitPrefixCheckRe.FindAllStringSubmatchIndex(body, -1) {
		ref := body[match[2]:match[3]]
		declScript, sigName := resolveSignalRef(ref, scriptName)
		key := declScript + "::" + sigName
		params, ok := idx[key]
		if !ok {
			continue
		}

		// Count args by finding balanced paren
		argsStart := match[1]
		depth := 1
		i := argsStart
		for i < len(body) && depth > 0 {
			switch body[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			i++
		}
		if depth != 0 {
			continue
		}
		argsStr := strings.TrimSpace(body[argsStart : i-1])
		argCount := 0
		if argsStr != "" {
			argCount = countTopLevelCommas(argsStr) + 1
		}

		if argCount != len(params) {
			c.add(emitArgCountMismatch(sigName, len(params), argCount, c.span(baseLine)))
		}
	}
}

func resolveSignalRef(ref, currentScript string) (string, string) {
	if before, after, ok := strings.Cut(ref, "::"); ok {
		return before, after
	}
	return currentScript, ref
}

func countTopLevelCommas(s string) int {
	depth := 0
	count := 0
	for _, ch := range s {
		switch ch {
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}
```

Wire `checkEmitArgCounts` into `checkScript` in `check.go`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add compiler/check/
git commit -m "add signal validation rules (connect resolution, emit arg count)"
```

---

## Task 6: Diagnostic Rules — Reactive and Watch Validation (Rules 11-12, 15)

**Files:**
- Modify: `compiler/check/rules.go`
- Modify: `compiler/check/guide.go`
- Modify: `compiler/check/check_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRule_WatchOnNonReactiveField(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "HUD",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "score", TypeExpr: "i32", Line: 1},
			},
			Watches: []ast.Watch{
				{Field: "self.score", Body: "println!(\"changed\");", Line: 3, BodyLine: 3},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "hud.fyx"})
	assertHasCode(t, diags, "F0011")
}

func TestRule_DerivedWithoutReactiveDep(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "HUD",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "base", TypeExpr: "f32", Default: "10.0", Line: 1},
				{Modifier: ast.FieldDerived, Name: "doubled", TypeExpr: "f32", Default: "self.base * 2.0", Line: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "hud.fyx"})
	assertHasCode(t, diags, "F0012")
}

func TestRule_EmptyScript(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Empty",
			Line: 1,
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "empty.fyx"})
	assertHasCode(t, diags, "F0015")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v -run "TestRule_(Watch|Derived|Empty)"`
Expected: FAIL

- [ ] **Step 3: Add guide messages**

Add to `compiler/check/guide.go`:

```go
func watchOnNonReactive(fieldExpr string, s span.Span) diag.Diagnostic {
	fieldName := strings.TrimPrefix(fieldExpr, "self.")
	return diag.Diagnostic{
		Code:     "F0011",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("watch on non-reactive field `%s`", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("You can only `watch` reactive or derived fields."),
			Explain: fmt.Sprintf("`%s` is not declared as `reactive` or `derived`, so Fyx can't track when it changes.", fieldName),
			Suggest: fmt.Sprintf(`To make it watchable, change the declaration to:

    reactive %s: TYPE = DEFAULT`, fieldName),
		},
	}
}

func derivedWithoutReactiveDep(fieldName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0012",
		Severity: diag.SeverityWarning,
		Message:  fmt.Sprintf("derived field `%s` doesn't reference any reactive fields", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("This derived field doesn't depend on any reactive fields."),
			Explain: "Derived fields recompute when their reactive dependencies change. Without any, this field never updates.",
			Suggest: fmt.Sprintf(`Either:
  - Reference a reactive field in the expression: derived %s: TYPE = self.REACTIVE_FIELD * 2
  - Or change it to a bare field if it doesn't need auto-updating`, fieldName),
		},
	}
}

func emptyScript(scriptName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0015",
		Severity: diag.SeverityWarning,
		Message:  fmt.Sprintf("script `%s` has no fields or handlers", scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "This script is empty — it won't do anything yet.",
			Explain: "Scripts need fields (data) or handlers (behavior) to be useful.",
			Suggest: fmt.Sprintf(`Try adding something:

    script %s {
        inspect speed: f32 = 5.0

        on update(ctx) {
            // runs every frame
        }
    }`, scriptName),
		},
	}
}
```

- [ ] **Step 4: Implement reactive/watch/empty rules**

Add to `compiler/check/rules.go`:

```go
func (c *checkCtx) checkWatches(script ast.Script) {
	reactiveFields := map[string]bool{}
	for _, f := range script.Fields {
		if f.Modifier == ast.FieldReactive || f.Modifier == ast.FieldDerived {
			reactiveFields[f.Name] = true
		}
	}

	for _, w := range script.Watches {
		fieldName := strings.TrimPrefix(w.Field, "self.")
		if !reactiveFields[fieldName] {
			c.add(watchOnNonReactive(w.Field, c.span(w.Line)))
		}
	}

	// Check derived fields reference at least one reactive
	for _, f := range script.Fields {
		if f.Modifier != ast.FieldDerived {
			continue
		}
		hasReactiveDep := false
		for _, rf := range script.Fields {
			if rf.Modifier == ast.FieldReactive || rf.Modifier == ast.FieldDerived {
				if strings.Contains(f.Default, "self."+rf.Name) {
					hasReactiveDep = true
					break
				}
			}
		}
		if !hasReactiveDep {
			c.add(derivedWithoutReactiveDep(f.Name, c.span(f.Line)))
		}
	}
}

func (c *checkCtx) checkEmpty(script ast.Script) {
	if len(script.Fields) == 0 && len(script.Handlers) == 0 && len(script.States) == 0 &&
		len(script.Signals) == 0 && len(script.Connects) == 0 && len(script.Watches) == 0 {
		c.add(emptyScript(script.Name, c.span(script.Line)))
	}
}
```

Wire `checkEmpty` into `checkScript` in `check.go`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add compiler/check/
git commit -m "add reactive, watch, and empty-script validation rules"
```

---

## Task 7: Wire Diagnostic Engine into fyxc CLI

**Files:**
- Modify: `cmd/fyxc/project.go`
- Modify: `cmd/fyxc/main.go` (add `--verbose` flag)

- [ ] **Step 1: Write failing test**

Add to `cmd/fyxc/main_test.go`:

```go
func TestVerboseFlag_Parsed(t *testing.T) {
	cmd, flagArgs, _ := parseArgs([]string{"check", "--verbose", "testdata"})
	if cmd != "check" {
		t.Errorf("expected cmd=check, got %s", cmd)
	}
	found := false
	for _, f := range flagArgs {
		if f == "--verbose" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --verbose in flagArgs, got %v", flagArgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/draco/work/fyrox-lang && go test ./cmd/fyxc/ -v -run TestVerboseFlag`
Expected: FAIL — `--verbose` is not in parseArgs allowlist, so it lands in posArgs.

- [ ] **Step 3: Add --verbose flag and wire diagnostics into compilation**

In `cmd/fyxc/main.go`, add `--verbose` to the flag set and `parseArgs`:

```go
verbose := fs.Bool("verbose", true, "Use guide-voice error messages (default: true)")
```

In `cmd/fyxc/project.go`, add a `checkPhase` after AST building but before transpilation:

```go
import "github.com/odvcencio/fyx/compiler/check"

// In compileProject, after building ASTs and signal index:
// transpiler.SignalIndex and check.SignalIndex are both map[string][]ast.Param,
// so signalIndex can be passed directly.
for i := range result.Files {
    file := &result.Files[i]
    diags := check.CheckFile(file.File, check.CheckOptions{
        FilePath:    file.SourcePath,
        SignalIndex: check.SignalIndex(signalIndex),
    })
    // Collect and return diagnostics
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/draco/work/fyrox-lang && go test ./cmd/fyxc/ -v -run TestVerboseFlag`
Expected: PASS

- [ ] **Step 5: Manual verification**

Create a test file with a known mistake and run:
```bash
echo 'script Player { inspect speed }' > /tmp/test.fyx
cd /home/draco/work/fyrox-lang && go run ./cmd/fyxc check /tmp/
```
Expected: Guide-voice error about missing type.

- [ ] **Step 6: Commit**

```bash
git add cmd/fyxc/main.go cmd/fyxc/project.go
git commit -m "wire semantic diagnostics into fyxc check/build pipeline"
```

---

## Task 8: LSP Server — Core Framework

**Files:**
- Create: `compiler/lsp/server.go`
- Create: `compiler/lsp/server_test.go`
- Create: `cmd/fyxc/lsp.go`
- Modify: `cmd/fyxc/main.go` (add "lsp" subcommand)

- [ ] **Step 1: Add LSP dependency**

Run: `cd /home/draco/work/fyrox-lang && go get go.lsp.dev/protocol@latest go.lsp.dev/jsonrpc2@latest go.lsp.dev/uri@latest`

If `go.lsp.dev/protocol` is too heavy or has issues, fall back to hand-rolled JSON-RPC (the subset we need is small: initialize, didOpen, didChange, publishDiagnostics, completion, hover). Check dependency weight and decide.

- [ ] **Step 2: Write failing test for LSP server initialization**

```go
// compiler/lsp/server_test.go
package lsp

import "testing"

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestServer_HandleDidOpen_PublishesDiagnostics(t *testing.T) {
	s := NewServer()
	diags := s.ValidateDocument("player.fyx", `script Player {
    inspect speed
}`)
	if len(diags) == 0 {
		t.Error("expected diagnostics for missing type, got none")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/lsp/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 4: Implement LSP server core**

```go
// compiler/lsp/server.go
package lsp

import (
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
}

// NewServer creates a new Fyx language server.
func NewServer() *Server {
	lang, err := grammargen.GenerateLanguage(grammar.FyxGrammar())
	if err != nil {
		panic("failed to generate grammar: " + err.Error())
	}
	return &Server{lang: lang}
}

// ValidateDocument parses and validates a single .fyx document.
func (s *Server) ValidateDocument(path string, content string) []diag.Diagnostic {
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
	return check.CheckFile(*file, check.CheckOptions{FilePath: path})
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/lsp/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add compiler/lsp/ go.mod go.sum
git commit -m "add LSP server core with document validation"
```

---

## Task 9: LSP Server — JSON-RPC Transport and Stdio

**Files:**
- Modify: `compiler/lsp/server.go` (add LSP protocol methods)
- Create: `cmd/fyxc/lsp.go`
- Modify: `cmd/fyxc/main.go`

This task implements the actual LSP protocol over stdio. The implementation depends on whether `go.lsp.dev/protocol` was usable from Task 8 Step 1. If not, implement a minimal JSON-RPC handler.

- [ ] **Step 1: Implement LSP stdio transport**

Create `cmd/fyxc/lsp.go`:

```go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/odvcencio/fyx/compiler/lsp"
)

func runLSP() error {
	server := lsp.NewServer()
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		msg, err := readLSPMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		responses := server.HandleMessage(msg)
		for _, resp := range responses {
			if err := writeLSPMessage(writer, resp); err != nil {
				return err
			}
		}
	}
}

func readLSPMessage(r *bufio.Reader) (json.RawMessage, error) {
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(val)
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeLSPMessage(w io.Writer, msg json.RawMessage) error {
	content := string(msg)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	_, err := io.WriteString(w, header+content)
	return err
}
```

- [ ] **Step 2: Add HandleMessage to LSP server**

Add to `compiler/lsp/server.go`:

```go
// HandleMessage processes a JSON-RPC message and returns responses.
func (s *Server) HandleMessage(raw json.RawMessage) []json.RawMessage {
	var req struct {
		ID     interface{} `json:"id,omitempty"`
		Method string      `json:"method"`
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
		return s.handleCompletion(req.ID, req.Params)
	case "textDocument/hover":
		return s.handleHover(req.ID, req.Params)
	case "shutdown":
		return []json.RawMessage{mustJSON(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": nil})}
	case "exit":
		return nil // caller should exit after receiving nil for "exit"
	default:
		return nil
	}
}
```

Implement each handler method. `handleInitialize` returns capabilities (completionProvider, hoverProvider, textDocumentSync: Full). `handleDidOpen`/`handleDidChange` call `ValidateDocument` and publish diagnostics. `handleCompletion` and `handleHover` delegate to catalog functions (Task 10).

- [ ] **Step 3: Wire "lsp" subcommand into main.go**

In `cmd/fyxc/main.go`, modify `parseArgs` and `main`:

```go
// In main(), before the flag set:
if cmd == "lsp" {
    if err := runLSP(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    return
}
```

Add `"lsp"` to the valid commands in `parseArgs`.

- [ ] **Step 4: Manual verification**

```bash
cd /home/draco/work/fyrox-lang && printf 'Content-Length: 58\r\n\r\n{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | go run ./cmd/fyxc lsp
```
Expected: JSON response with Content-Length header and capabilities object. Note: the Content-Length value (58) must match the exact byte length of the JSON body.

- [ ] **Step 5: Commit**

```bash
git add cmd/fyxc/lsp.go cmd/fyxc/main.go compiler/lsp/
git commit -m "add LSP stdio transport and message handling"
```

---

## Task 10: LSP — Keyword Completions and Hover

**Files:**
- Create: `compiler/lsp/completions.go`
- Create: `compiler/lsp/hover.go`
- Create: `compiler/lsp/completions_test.go`

- [ ] **Step 1: Write failing test**

```go
// compiler/lsp/completions_test.go
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

	required := []string{"script", "inspect", "node", "reactive", "derived", "signal", "on update", "on start", "component", "system", "state", "timer", "emit", "connect", "watch"}
	for _, kw := range required {
		if !found[kw] {
			t.Errorf("missing completion for %q", kw)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/lsp/ -v -run "Test(Completion|Hover)"`
Expected: FAIL

- [ ] **Step 3: Implement completion catalog**

```go
// compiler/lsp/completions.go
package lsp

// CompletionItem is a simplified LSP completion item.
type CompletionItem struct {
	Label      string
	Detail     string
	InsertText string
	Kind       int // 14=keyword, 15=snippet
}

// CompletionItems returns all Fyx keyword completions.
func CompletionItems() []CompletionItem {
	return []CompletionItem{
		{Label: "script", Detail: "Declare a new script", InsertText: "script ${1:Name} {\n    $0\n}", Kind: 15},
		{Label: "inspect", Detail: "Editor-visible field", InsertText: "inspect ${1:name}: ${2:f32} = ${3:0.0}", Kind: 15},
		{Label: "node", Detail: "Scene node binding", InsertText: "node ${1:name}: ${2:Node} = \"${3:Path}\"", Kind: 15},
		{Label: "nodes", Detail: "Scene node collection", InsertText: "nodes ${1:name}: ${2:Node} = \"${3:Path/*}\"", Kind: 15},
		{Label: "resource", Detail: "Resource binding", InsertText: "resource ${1:name}: ${2:Model} = \"res://${3:path}\"", Kind: 15},
		{Label: "timer", Detail: "Countdown timer field", InsertText: "timer ${1:name} = ${2:1.0}", Kind: 15},
		{Label: "reactive", Detail: "Change-tracked field", InsertText: "reactive ${1:name}: ${2:f32} = ${3:0.0}", Kind: 15},
		{Label: "derived", Detail: "Computed from reactives", InsertText: "derived ${1:name}: ${2:bool} = ${3:self.field > 0}", Kind: 15},
		{Label: "signal", Detail: "Declare a typed signal", InsertText: "signal ${1:name}(${2:param: Type})", Kind: 15},
		{Label: "on update", Detail: "Runs every frame", InsertText: "on update(ctx) {\n    $0\n}", Kind: 15},
		{Label: "on start", Detail: "Runs at scene start", InsertText: "on start(ctx) {\n    $0\n}", Kind: 15},
		{Label: "on init", Detail: "Runs at creation", InsertText: "on init(ctx) {\n    $0\n}", Kind: 15},
		{Label: "on event", Detail: "OS/window events", InsertText: "on event(event, ctx) {\n    $0\n}", Kind: 15},
		{Label: "on deinit", Detail: "Cleanup handler", InsertText: "on deinit(ctx) {\n    $0\n}", Kind: 15},
		{Label: "state", Detail: "State machine state", InsertText: "state ${1:name} {\n    on enter {\n        $0\n    }\n    on update {\n    }\n}", Kind: 15},
		{Label: "component", Detail: "ECS component", InsertText: "component ${1:Name} {\n    ${2:field}: ${3:Type}\n}", Kind: 15},
		{Label: "system", Detail: "ECS system", InsertText: "system ${1:name}(dt: f32) {\n    query(${2:entity: Entity, comp: &mut Type}) {\n        $0\n    }\n}", Kind: 15},
		{Label: "emit", Detail: "Emit a signal", InsertText: "emit ${1:signal_name}(${2:args});", Kind: 15},
		{Label: "connect", Detail: "Subscribe to signal", InsertText: "connect ${1:Script}::${2:signal}(${3:param}) {\n    $0\n}", Kind: 15},
		{Label: "watch", Detail: "React to field changes", InsertText: "watch self.${1:field} {\n    $0\n}", Kind: 15},
		{Label: "spawn", Detail: "Spawn prefab", InsertText: "let ${1:entity} = spawn ${2:self.prefab} at ${3:position};", Kind: 15},
		{Label: "query", Detail: "ECS query loop", InsertText: "query(${1:entity: Entity, comp: &Type}) {\n    $0\n}", Kind: 15},
	}
}
```

- [ ] **Step 4: Implement hover catalog**

```go
// compiler/lsp/hover.go
package lsp

import "strings"

var hoverDocs = map[string]string{
	"script": "**script** — A gameplay object attached to a scene node.\n\nScripts hold data (fields) and behavior (handlers). They're the main building block for game logic in Fyx.\n\n```fyx\nscript Player {\n    inspect speed: f32 = 5.0\n    on update(ctx) { }\n}\n```",

	"inspect": "**inspect** — A field visible in the Fyrox editor.\n\nInspect fields show up in the editor's inspector panel. You can tweak their values while the game runs.\n\n- `f32` — decimal number\n- `i32` — whole number\n- `bool` — true/false\n- `String` — text\n- `Vector3` — 3D position\n\n```fyx\ninspect speed: f32 = 5.0\n```",

	"node": "**node** — A reference to a scene node.\n\nBinds to a node in your scene tree by path. Resolved automatically when the script starts.\n\n```fyx\nnode muzzle: Node = \"Turret/Muzzle\"\n```\n\nUse `.position()`, `.forward()`, and other shortcuts on node fields.",

	"nodes": "**nodes** — A collection of scene nodes.\n\nBinds to multiple nodes using a wildcard path. Call methods on all of them at once.\n\n```fyx\nnodes digits: Text = \"UI/Digits/*\"\n```",

	"resource": "**resource** — A loaded asset (model, texture, sound).\n\nLoaded from a `res://` path via the resource manager.\n\n```fyx\nresource bullet: Model = \"res://models/bullet.rgs\"\n```",

	"timer": "**timer** — A countdown timer field.\n\nAutomatically counts down each frame. Use `.ready` to check if done, `.reset()` to restart.\n\n```fyx\ntimer cooldown = 0.5\n\non update(ctx) {\n    if cooldown.ready {\n        // fire!\n        cooldown.reset();\n    }\n}\n```",

	"reactive": "**reactive** — A field that tracks changes.\n\nFyx automatically detects when this field's value changes. Used with `derived` and `watch`.\n\n```fyx\nreactive health: f32 = 100.0\n```",

	"derived": "**derived** — A computed value that updates when its dependencies change.\n\nRecalculated only when the reactive fields it references change.\n\n```fyx\nreactive health: f32 = 100.0\nderived is_dead: bool = self.health <= 0.0\n```",

	"signal": "**signal** — A typed event that other scripts can listen for.\n\n```fyx\nsignal died(position: Vector3)\n\n// Emit it:\nemit died(self.position());\n```",

	"emit": "**emit** — Send a signal.\n\nBroadcasts globally or targets a specific node.\n\n```fyx\nemit fired(origin, direction);\nemit Enemy::damaged(10.0) to target;\n```",

	"connect": "**connect** — Subscribe to another script's signal.\n\n```fyx\nconnect Enemy::died(pos) {\n    self.score += 100;\n}\n```",

	"watch": "**watch** — Run code when a reactive/derived field changes.\n\n```fyx\nwatch self.is_dead {\n    emit died(self.position());\n}\n```",

	"state": "**state** — A state machine state.\n\nScripts can have multiple states with enter/update/exit handlers. Switch states with `go`.\n\n```fyx\nstate idle {\n    on update { if see_player() { go alert; } }\n}\nstate alert {\n    on enter { play_alarm(); }\n}\n```",

	"component": "**component** — An ECS data structure.\n\nLightweight data attached to entities. Used with systems and queries.\n\n```fyx\ncomponent Velocity {\n    linear: Vector3\n    angular: Vector3\n}\n```",

	"system": "**system** — An ECS system that processes entities.\n\nRuns queries over entities with matching components.\n\n```fyx\nsystem move(dt: f32) {\n    query(pos: &mut Transform, vel: &Velocity) {\n        pos.translate(vel.linear * dt);\n    }\n}\n```",

	"query": "**query** — Loop over entities with specific components.\n\nUsed inside systems to process matching entities.\n\n```fyx\nquery(entity: Entity, hp: &mut Health) {\n    if hp.current <= 0.0 { despawn(entity); }\n}\n```",

	"dt": "**dt** — Delta time (seconds since last frame).\n\nAvailable in `on update` handlers. Use it to make movement frame-rate independent.\n\n```fyx\non update(ctx) {\n    self.position.x += self.speed * dt;\n}\n```",

	"spawn": "**spawn** — Create an instance of a prefab in the scene.\n\n```fyx\nlet bullet = spawn self.bullet_prefab at muzzle_pos;\nlet bullet = spawn self.prefab at pos lifetime 3.0;\n```",
}

// HoverInfo returns markdown documentation for a Fyx keyword.
func HoverInfo(keyword string) string {
	keyword = strings.TrimSpace(strings.ToLower(keyword))
	return hoverDocs[keyword]
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/lsp/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add compiler/lsp/completions.go compiler/lsp/hover.go compiler/lsp/completions_test.go
git commit -m "add keyword completions and hover documentation for LSP"
```

---

## Task 11: VS Code Extension — Language Client

**Files:**
- Modify: `editors/vscode/package.json`
- Create: `editors/vscode/src/extension.ts`
- Create: `editors/vscode/tsconfig.json`
- Create: `editors/vscode/.vscodeignore`

- [ ] **Step 1: Initialize the TypeScript extension project**

```bash
cd /home/draco/work/fyrox-lang/editors/vscode
npm init -y
npm install vscode-languageclient vscode-languageserver-protocol
npm install -D @types/vscode typescript
```

- [ ] **Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "module": "commonjs",
    "target": "ES2020",
    "outDir": "out",
    "rootDir": "src",
    "lib": ["ES2020"],
    "sourceMap": true,
    "strict": true
  },
  "exclude": ["node_modules"]
}
```

- [ ] **Step 3: Create the extension entry point**

```typescript
// editors/vscode/src/extension.ts
import * as path from 'path';
import { workspace, ExtensionContext, window } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind,
} from 'vscode-languageclient/node';

let client: LanguageClient;

export function activate(context: ExtensionContext) {
    const config = workspace.getConfiguration('fyx');
    const serverCommand = config.get<string>('serverPath', 'fyxc');

    const serverOptions: ServerOptions = {
        run: { command: serverCommand, args: ['lsp'], transport: TransportKind.stdio },
        debug: { command: serverCommand, args: ['lsp'], transport: TransportKind.stdio },
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'fyx' }],
        synchronize: {
            fileEvents: workspace.createFileSystemWatcher('**/*.fyx'),
        },
    };

    client = new LanguageClient(
        'fyxLanguageServer',
        'Fyx Language Server',
        serverOptions,
        clientOptions
    );

    client.start();
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
```

- [ ] **Step 4: Update package.json for LSP client**

Update `editors/vscode/package.json` to add:
- `"main": "./out/extension.js"`
- `"activationEvents": ["onLanguage:fyx"]`
- `"scripts": { "compile": "tsc -p .", "watch": "tsc -watch -p ." }`
- Configuration contribution for `fyx.serverPath` and `fyx.verboseErrors`
- Dependencies on `vscode-languageclient`

- [ ] **Step 5: Build and verify**

```bash
cd /home/draco/work/fyrox-lang/editors/vscode && npm run compile
```
Expected: `out/extension.js` created without errors.

- [ ] **Step 6: Create .vscodeignore**

```
src/**
node_modules/**
tsconfig.json
*.ts
!out/**
```

- [ ] **Step 7: Commit**

```bash
git add editors/vscode/
git commit -m "upgrade VS Code extension with LSP language client"
```

---

## Task 12: Integration Test — LSP End-to-End

**Files:**
- Create: `compiler/lsp/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
// compiler/lsp/integration_test.go
package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLSP_DidOpen_ProducesDiagnostics(t *testing.T) {
	s := NewServer()

	// Simulate didOpen with a file that has a missing field type
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///test/player.fyx",
			"languageId": "fyx",
			"version":    1,
			"text":       "script Player {\n    inspect speed\n}",
		},
	})

	responses := s.HandleMessage(mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params":  json.RawMessage(params),
	}))

	if len(responses) == 0 {
		t.Fatal("expected diagnostic notification, got none")
	}

	var notif struct {
		Method string `json:"method"`
		Params struct {
			Diagnostics []struct {
				Message string `json:"message"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal(responses[0], &notif); err != nil {
		t.Fatalf("failed to parse notification: %v", err)
	}
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Errorf("expected publishDiagnostics, got %s", notif.Method)
	}
	if len(notif.Params.Diagnostics) == 0 {
		t.Error("expected at least one diagnostic")
	}
}

func TestLSP_DidOpen_ValidFile_NoDiagnostics(t *testing.T) {
	s := NewServer()

	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///test/player.fyx",
			"languageId": "fyx",
			"version":    1,
			"text":       "script Player {\n    inspect speed: f32 = 5.0\n    on update(ctx) {\n        self.speed += 1.0;\n    }\n}",
		},
	})

	responses := s.HandleMessage(mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params":  json.RawMessage(params),
	}))

	if len(responses) == 0 {
		t.Fatal("expected diagnostic notification")
	}

	var notif struct {
		Params struct {
			Diagnostics []json.RawMessage `json:"diagnostics"`
		} `json:"params"`
	}
	json.Unmarshal(responses[0], &notif)
	if len(notif.Params.Diagnostics) != 0 {
		t.Errorf("expected zero diagnostics for valid file, got %d", len(notif.Params.Diagnostics))
	}
}

func TestLSP_Completion_ReturnsKeywords(t *testing.T) {
	s := NewServer()

	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test/player.fyx"},
		"position":     map[string]interface{}{"line": 0, "character": 0},
	})

	responses := s.HandleMessage(mustJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/completion",
		"params":  json.RawMessage(params),
	}))

	if len(responses) == 0 {
		t.Fatal("expected completion response")
	}

	resp := string(responses[0])
	if !strings.Contains(resp, "script") {
		t.Error("completion response should include 'script' keyword")
	}
}
```

- [ ] **Step 2: Run test**

Run: `cd /home/draco/work/fyrox-lang && go test ./compiler/lsp/ -v -run TestLSP_`
Expected: PASS (if prior tasks are complete) or FAIL (fix and iterate).

- [ ] **Step 3: Commit**

```bash
git add compiler/lsp/integration_test.go
git commit -m "add LSP integration tests"
```

---

## Task 13: Multi-File Scale Test

**Files:**
- Create: `testdata/scale/` (20+ .fyx files with cross-references)
- Modify: `integration_test.go` (add scale test)

- [ ] **Step 1: Create 20+ .fyx files that cross-reference signals and components**

Create a `testdata/scale/` directory with files like:
- `player.fyx` — script with signals, connects to enemy signals
- `enemy.fyx` — script with signals, connects to player signals
- `weapon.fyx` — script with signals, spawns projectile components
- `projectile.fyx` — component + system
- `hud.fyx` — script with reactive/derived/watch, connects to player signals
- `health.fyx` — component + system
- `score.fyx` — script connecting to multiple signals
- `audio.fyx` — script connecting to weapon/enemy signals
- `fx.fyx` — script with spawn + lifetime
- `camera.fyx` — script with node bindings
- `input.fyx` — script with event handlers + state machine
- `ai/patrol.fyx` — nested module, state machine
- `ai/attack.fyx` — nested module, signals
- `ai/flee.fyx` — nested module
- `ui/healthbar.fyx` — UI script, connects to health signals
- `ui/ammo.fyx` — UI script, nodes collection
- `ui/minimap.fyx` — UI script, node bindings
- `world/spawner.fyx` — ECS spawn + component
- `world/physics.fyx` — system
- `world/cleanup.fyx` — system with despawn

Each file should be small (10-30 lines) and reference at least one signal or component from another file.

- [ ] **Step 2: Write integration test**

This test goes in `cmd/fyxc/main_test.go` (package `main`) since `compileProject` is unexported. Alternatively, create `cmd/fyxc/scale_test.go`.

```go
// cmd/fyxc/scale_test.go
package main

import "testing"

func TestScaleProject_20Files(t *testing.T) {
	result, err := compileProject("../../testdata/scale")
	if err != nil {
		t.Fatalf("scale project compilation failed: %v", err)
	}

	if len(result.Files) < 20 {
		t.Errorf("expected at least 20 files, got %d", len(result.Files))
	}

	// Verify cross-file signal resolution works
	if result.TotalScripts < 10 {
		t.Errorf("expected at least 10 scripts, got %d", result.TotalScripts)
	}

	// Verify all files transpile without error
	for _, file := range result.Files {
		if file.Output.Code == "" {
			t.Errorf("empty output for %s", file.SourcePath)
		}
	}
}
```

- [ ] **Step 3: Run the test**

Run: `cd /home/draco/work/fyrox-lang && go test ./cmd/fyxc/ -v -run TestScaleProject_20Files`
Expected: PASS — all 20+ files compile with cross-references resolved.

- [ ] **Step 4: Run diagnostics on scale project**

Run: `cd /home/draco/work/fyrox-lang && go run ./cmd/fyxc check testdata/scale --verbose`
Expected: No errors (or only intentional warnings), guide-voice format.

- [ ] **Step 5: Commit**

```bash
git add testdata/scale/ integration_test.go
git commit -m "add 20-file scale test for multi-file compilation"
```

---

## Task 14: Tutorial — "Your First Fyx Game"

**Files:**
- Create: `docs/tutorial/01-setup.md`
- Create: `docs/tutorial/02-first-script.md`
- Create: `docs/tutorial/03-make-it-move.md`
- Create: `docs/tutorial/04-editor-fields.md`
- Create: `docs/tutorial/05-input.md`
- Create: `docs/tutorial/06-state-machines.md`
- Create: `docs/tutorial/07-signals.md`
- Create: `docs/tutorial/08-whats-next.md`

Each tutorial page should be written to be standalone-readable but link forward/backward. The tone should be encouraging and assume no Rust knowledge. Code examples should be complete — the reader should be able to copy-paste each step.

- [ ] **Step 1: Write 01-setup.md**

Cover: installing Go, installing fyxc (`go install`), installing VS Code extension, creating a project directory, verifying `fyxc check` works.

- [ ] **Step 2: Write 02-first-script.md**

Cover: `script Player { inspect speed: f32 = 5.0 }`, what each keyword means, running `fyxc build`, seeing the generated Rust (optional: look but don't worry about it).

- [ ] **Step 3: Write 03-make-it-move.md**

Cover: `on update(ctx) { ... }`, `dt`, `self.node`, moving a node. Explain frames and delta time.

- [ ] **Step 4: Write 04-editor-fields.md**

Cover: `inspect` fields in Fyrox editor, tweaking values while running. `node` field for scene bindings.

- [ ] **Step 5: Write 05-input.md**

Cover: `on event`, keyboard input, `ElementState::Pressed`, moving with WASD.

- [ ] **Step 6: Write 06-state-machines.md**

Cover: `state idle { on update { ... } }`, `go`, enter/exit handlers. Example: idle/walking/jumping.

- [ ] **Step 7: Write 07-signals.md**

Cover: `signal`, `emit`, `connect`. Example: Player emits `took_damage`, HUD connects and updates display.

- [ ] **Step 8: Write 08-whats-next.md**

Cover: reactive/derived/watch (brief), ECS (brief), timers, spawn, raw Rust escape hatch. Links to future example gallery and reference.

- [ ] **Step 9: Commit**

```bash
git add docs/tutorial/
git commit -m "add 'Your First Fyx Game' tutorial (8 chapters)"
```

---

## Task 15: Final Integration — Run Full Test Suite

- [ ] **Step 1: Run all existing tests**

```bash
cd /home/draco/work/fyrox-lang && go test ./... -v
```
Expected: All existing tests still pass. No regressions.

- [ ] **Step 2: Run diagnostic engine tests**

```bash
cd /home/draco/work/fyrox-lang && go test ./compiler/check/ -v
```
Expected: All 15 diagnostic rules pass.

- [ ] **Step 3: Run LSP tests**

```bash
cd /home/draco/work/fyrox-lang && go test ./compiler/lsp/ -v
```
Expected: Server creation, document validation, completion, hover all pass.

- [ ] **Step 4: Run scale test**

```bash
cd /home/draco/work/fyrox-lang && go test -v -run TestScaleProject
```
Expected: 20+ files compile correctly.

- [ ] **Step 5: Build VS Code extension**

```bash
cd /home/draco/work/fyrox-lang/editors/vscode && npm run compile
```
Expected: Extension compiles without errors.

- [ ] **Step 6: Manual smoke test**

1. Build fyxc: `go build -o /tmp/fyxc ./cmd/fyxc`
2. Open VS Code in a test project with `.fyx` files
3. Verify: syntax highlighting, red squiggles on errors, hover on keywords, keyword completions
4. Verify: guide-voice errors appear in the Problems panel

- [ ] **Step 7: Commit any final fixes**

```bash
git add -A
git commit -m "final integration fixes for beginner experience vertical slice"
```
