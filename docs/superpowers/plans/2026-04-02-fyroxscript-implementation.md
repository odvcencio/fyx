# FyroxScript Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a transpiler that compiles FyroxScript (`.fyx`) files to idiomatic Fyrox Rust, with a tree-sitter grammar, AST, codegen pipeline, and CLI build tool.

**Architecture:** The compiler is a Go program that uses gotreesitter's grammargen to extend the Rust grammar with FyroxScript productions. The parser produces a concrete syntax tree, which a Go transpiler walks to emit `.rs` files. A lightweight ECS runtime library (Rust crate) ships alongside generated code.

**Tech Stack:** Go (compiler toolchain, grammar, transpiler), gotreesitter/grammargen (parser generation), Rust (generated output + ECS runtime crate), Fyrox 1.x (target engine)

**Spec:** `docs/superpowers/specs/2026-04-02-fyroxscript-design.md`

---

## File Structure

```
fyrox-lang/
├── grammar/
│   ├── fyroxscript.go          # Grammar definition extending Rust
│   └── fyroxscript_test.go     # Grammar parse tests
├── ast/
│   ├── ast.go                  # AST node types
│   ├── builder.go              # CST → AST conversion
│   └── builder_test.go         # AST builder tests
├── transpiler/
│   ├── script.go               # Script block → Rust struct + ScriptTrait
│   ├── script_test.go
│   ├── signals.go              # Event signals → message structs + dispatch
│   ├── signals_test.go
│   ├── reactive.go             # Reactive/derived/watch → dirty-tracking
│   ├── reactive_test.go
│   ├── ecs.go                  # Component/system/query → Rust code
│   ├── ecs_test.go
│   ├── nodes.go                # Node/resource field resolution
│   ├── nodes_test.go
│   ├── body.go                 # Handler body rewriting (self.position(), spawn...at)
│   ├── body_test.go
│   ├── emit.go                 # Rust code emitter (formatting, imports)
│   └── emit_test.go
├── cmd/
│   └── fyxc/
│       └── main.go             # CLI: fyxc build, fyxc watch
├── runtime/                    # Rust crate shipped with generated code
│   ├── Cargo.toml
│   └── src/
│       ├── lib.rs              # Re-exports
│       └── ecs.rs              # Sparse-set ECS storage
├── testdata/
│   ├── minimal.fyx             # Minimal script
│   ├── signals.fyx             # Event signals test
│   ├── reactive.fyx            # Reactive signals test
│   ├── ecs.fyx                 # ECS test
│   ├── full.fyx                # Complete example from spec
│   └── golden/                 # Expected .rs output files
│       ├── minimal.rs
│       ├── signals.rs
│       ├── reactive.rs
│       ├── ecs.rs
│       └── full.rs
├── go.mod
├── go.sum
└── docs/
    └── superpowers/
        ├── specs/
        │   └── 2026-04-02-fyroxscript-design.md
        └── plans/
            └── 2026-04-02-fyroxscript-implementation.md
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`, `go.sum`
- Create: `grammar/fyroxscript.go`
- Create: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /home/draco/work/fyrox-lang
go mod init github.com/odvcencio/fyrox-lang
go get github.com/odvcencio/gotreesitter
go get github.com/odvcencio/gotreesitter/grammargen
```

- [ ] **Step 2: Create minimal grammar file**

Create `grammar/fyroxscript.go`:

```go
package grammar

import (
    "github.com/odvcencio/gotreesitter/grammargen"
)

// FyroxScriptGrammar returns a grammar that extends Rust with
// FyroxScript-specific productions: script, signal, component, system, etc.
func FyroxScriptGrammar() *grammargen.Grammar {
    g := grammargen.NewGrammar("fyroxscript")

    // Placeholder — start with source_file containing items
    g.Define("source_file", grammargen.Repeat(grammargen.Sym("_item")))
    g.Define("_item", grammargen.Choice(
        grammargen.Sym("script_declaration"),
    ))
    g.Define("script_declaration", grammargen.Seq(
        grammargen.Str("script"),
        grammargen.Field("name", grammargen.Sym("identifier")),
        grammargen.Str("{"),
        grammargen.Str("}"),
    ))
    g.Define("identifier", grammargen.Pat(`[a-zA-Z_][a-zA-Z0-9_]*`))

    g.SetExtras(grammargen.Pat(`\s`))

    return g
}
```

- [ ] **Step 3: Write test that parses `script Player {}`**

Create `grammar/fyroxscript_test.go`:

```go
package grammar

import (
    "strings"
    "testing"

    "github.com/odvcencio/gotreesitter/grammargen"
    gotreesitter "github.com/odvcencio/gotreesitter"
)

func generateLang(t *testing.T) *gotreesitter.Language {
    t.Helper()
    g := FyroxScriptGrammar()
    lang, err := grammargen.GenerateLanguage(g)
    if err != nil {
        t.Fatalf("generate: %v", err)
    }
    return lang
}

func TestParseEmptyScript(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)
    tree, err := parser.Parse([]byte("script Player {}"))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    if !strings.Contains(sexpr, "script_declaration") {
        t.Errorf("expected script_declaration node, got: %s", sexpr)
    }
}
```

- [ ] **Step 4: Run test**

```bash
cd /home/draco/work/fyrox-lang
go test ./grammar/ -v -run TestParseEmptyScript
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 2: Script Fields Grammar

**Files:**
- Modify: `grammar/fyroxscript.go`
- Modify: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Write failing test for script with inspect/node/resource fields**

Add to `grammar/fyroxscript_test.go`:

```go
func TestParseScriptFields(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)

    input := `script Player {
    inspect speed: f32 = 10.0
    inspect jump_force: f32 = 5.0
    node camera: Camera3D = "Camera3D"
    resource footstep: SoundBuffer = "res://audio/footstep.wav"
    move_dir: Vector3
}`
    tree, err := parser.Parse([]byte(input))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    for _, expected := range []string{"inspect_field", "node_field", "resource_field", "bare_field"} {
        if !strings.Contains(sexpr, expected) {
            t.Errorf("expected %q node, got: %s", expected, sexpr)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./grammar/ -v -run TestParseScriptFields
```

Expected: FAIL — missing field rules

- [ ] **Step 3: Add field grammar rules**

Update `grammar/fyroxscript.go` — add type expressions, field declarations with modifiers, and default values. Add `inspect_field`, `node_field`, `nodes_field`, `resource_field`, `bare_field` rules with `Field()` annotations for name, type, and default value. Add `type_expression` as identifier with optional generic params and `::` paths.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./grammar/ -v -run TestParseScriptFields
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 3: Lifecycle Handlers Grammar

**Files:**
- Modify: `grammar/fyroxscript.go`
- Modify: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Write failing test for on init/update/event handlers**

```go
func TestParseLifecycleHandlers(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)

    input := `script Player {
    inspect speed: f32 = 10.0

    on init(ctx) {
        let x = 5;
    }

    on update(ctx) {
        self.speed += 1.0;
    }

    on event(ev: KeyboardInput, ctx) {
    }
}`
    tree, err := parser.Parse([]byte(input))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    if !strings.Contains(sexpr, "lifecycle_handler") {
        t.Errorf("expected lifecycle_handler node, got: %s", sexpr)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Add lifecycle handler grammar rules**

Add `lifecycle_handler` rule: `on` + handler kind (`init`/`start`/`update`/`deinit`/`event`/`message`) + parameter list + block. The block body contains Rust statements — use a balanced-brace rule that captures everything between `{` and `}` as raw content, since FyroxScript passes Rust through verbatim.

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 4: Event Signals Grammar

**Files:**
- Modify: `grammar/fyroxscript.go`
- Modify: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Write failing test for signal/emit/connect**

```go
func TestParseSignals(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)

    input := `script Enemy {
    signal died(position: Vector3)
    signal damaged(amount: f32, source: Handle<Node>)

    on update(ctx) {
        emit died(self.position());
        emit damaged(10.0, ctx.handle) to target;
    }
}

script ScoreTracker {
    connect Enemy::died(pos) {
        self.score += 100;
    }
}`
    tree, err := parser.Parse([]byte(input))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    for _, expected := range []string{"signal_declaration", "connect_block"} {
        if !strings.Contains(sexpr, expected) {
            t.Errorf("expected %q, got: %s", expected, sexpr)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Add signal grammar rules**

Add `signal_declaration`, `emit_statement` (with optional `to` target), and `connect_block` rules. `emit` and `connect` reference signal names via `ScriptName::signal_name` path syntax.

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 5: Reactive Signals Grammar

**Files:**
- Modify: `grammar/fyroxscript.go`
- Modify: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Write failing test for reactive/derived/watch**

```go
func TestParseReactiveSignals(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)

    input := `script HUD {
    reactive health: f32 = 100.0
    derived health_pct: f32 = self.health / 100.0
    derived is_critical: bool = self.health < 20.0

    watch self.is_critical {
        do_something();
    }
}`
    tree, err := parser.Parse([]byte(input))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    for _, expected := range []string{"reactive_field", "derived_field", "watch_block"} {
        if !strings.Contains(sexpr, expected) {
            t.Errorf("expected %q, got: %s", expected, sexpr)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Add reactive grammar rules**

Add `reactive_field`, `derived_field` (with expression default), and `watch_block` (watches `self.field_name` + block body).

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 6: ECS Grammar

**Files:**
- Modify: `grammar/fyroxscript.go`
- Modify: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Write failing test for component/system/query**

```go
func TestParseECS(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)

    input := `component Velocity {
    linear: Vector3
    angular: Vector3
}

system move_projectiles(dt: f32) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt);
    }
}

system expire {
    query(entity: Entity, proj: &mut Projectile) {
        if proj.lifetime <= 0.0 {
            despawn(entity);
        }
    }
}`
    tree, err := parser.Parse([]byte(input))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    for _, expected := range []string{"component_declaration", "system_declaration", "query_block"} {
        if !strings.Contains(sexpr, expected) {
            t.Errorf("expected %q, got: %s", expected, sexpr)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Add ECS grammar rules**

Add `component_declaration` (like a struct with bare fields), `system_declaration` (name + optional injected params + body), and `query_block` (component parameter list with `&`/`&mut` references + body).

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 7: Rust Passthrough Grammar

**Files:**
- Modify: `grammar/fyroxscript.go`
- Modify: `grammar/fyroxscript_test.go`

- [ ] **Step 1: Write failing test for mixed FyroxScript + raw Rust**

```go
func TestParseRustPassthrough(t *testing.T) {
    lang := generateLang(t)
    parser := gotreesitter.NewParser(lang)

    input := `use fyrox::prelude::*;

fn helper(x: f32) -> f32 {
    x * 2.0
}

script Player {
    inspect speed: f32 = 10.0

    on update(ctx) {
        let s = helper(self.speed);
    }
}

struct CustomData {
    value: i32,
}

impl CustomData {
    fn new() -> Self {
        Self { value: 0 }
    }
}`
    tree, err := parser.Parse([]byte(input))
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Errorf("parse tree contains ERROR: %s", sexpr)
    }
    if !strings.Contains(sexpr, "script_declaration") {
        t.Errorf("expected script_declaration, got: %s", sexpr)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Add Rust passthrough to `_item`**

Add a `rust_item` rule to `_item` that captures any top-level Rust construct (fn, struct, enum, impl, use, mod, type, const, static, trait, pub, extern, unsafe, async, macro). This uses balanced-brace capture for bodies. The `_item` choice becomes: `script_declaration | component_declaration | system_declaration | rust_item`.

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 8: AST Types

**Files:**
- Create: `ast/ast.go`
- Create: `ast/ast_test.go`

- [ ] **Step 1: Define AST node types**

Create `ast/ast.go` with Go types representing the FyroxScript AST:

```go
package ast

// File is the root AST node — one per .fyx file.
type File struct {
    Scripts    []Script
    Components []Component
    Systems    []System
    RustItems  []RustItem  // passthrough Rust code
}

type Script struct {
    Name       string
    Fields     []Field
    Handlers   []Handler
    Signals    []Signal
    Connects   []Connect
    Watches    []Watch
}

type Field struct {
    Modifier   FieldModifier  // Inspect, Node, Nodes, Resource, Reactive, Derived, Bare
    Name       string
    TypeExpr   string         // raw type expression as string
    Default    string         // raw default expression (empty if none)
}

type FieldModifier int
const (
    FieldBare FieldModifier = iota
    FieldInspect
    FieldNode
    FieldNodes
    FieldResource
    FieldReactive
    FieldDerived
)

type Handler struct {
    Kind    HandlerKind  // Init, Start, Update, Deinit, Event, Message
    Params  []Param
    Body    string       // raw Rust body
}

type HandlerKind int
const (
    HandlerInit HandlerKind = iota
    HandlerStart
    HandlerUpdate
    HandlerDeinit
    HandlerEvent
    HandlerMessage
)

type Param struct {
    Name     string
    TypeExpr string  // empty for ctx (inferred)
}

type Signal struct {
    Name   string
    Params []Param
}

type Connect struct {
    ScriptName string
    SignalName string
    Params     []string  // binding names
    Body       string
}

type Watch struct {
    Field string   // e.g. "self.is_critical"
    Body  string
}

type Component struct {
    Name   string
    Fields []Field  // always Bare modifier
}

type System struct {
    Name       string
    Params     []Param     // injected params (dt, etc.)
    Queries    []Query
    Body       string      // non-query body code
}

type Query struct {
    Params []QueryParam
    Body   string
}

type QueryParam struct {
    Name     string
    Mutable  bool
    TypeExpr string
}

type RustItem struct {
    Source string  // raw Rust source, emitted unchanged
}
```

- [ ] **Step 2: Write construction test**

Create `ast/ast_test.go`:

```go
package ast

import "testing"

func TestScriptConstruction(t *testing.T) {
    s := Script{
        Name: "Player",
        Fields: []Field{
            {Modifier: FieldInspect, Name: "speed", TypeExpr: "f32", Default: "10.0"},
        },
        Handlers: []Handler{
            {Kind: HandlerUpdate, Params: []Param{{Name: "ctx"}}, Body: "self.speed += 1.0;"},
        },
    }
    if s.Name != "Player" {
        t.Errorf("expected Player, got %s", s.Name)
    }
    if len(s.Fields) != 1 {
        t.Errorf("expected 1 field, got %d", len(s.Fields))
    }
}
```

- [ ] **Step 3: Run test**

```bash
go test ./ast/ -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
buckley commit --yes --min
```

---

## Task 9: CST → AST Builder

**Files:**
- Create: `ast/builder.go`
- Create: `ast/builder_test.go`

- [ ] **Step 1: Write failing test — parse .fyx source to AST**

Create `ast/builder_test.go`:

```go
package ast

import (
    "testing"

    "github.com/odvcencio/fyrox-lang/grammar"
    "github.com/odvcencio/gotreesitter/grammargen"
    gotreesitter "github.com/odvcencio/gotreesitter"
)

func lang(t *testing.T) *gotreesitter.Language {
    t.Helper()
    g := grammar.FyroxScriptGrammar()
    l, err := grammargen.GenerateLanguage(g)
    if err != nil {
        t.Fatalf("generate: %v", err)
    }
    return l
}

func TestBuildMinimalScript(t *testing.T) {
    l := lang(t)
    source := []byte(`script Player {
    inspect speed: f32 = 10.0
    on update(ctx) {
        self.speed += 1.0;
    }
}`)
    file, err := BuildAST(l, source)
    if err != nil {
        t.Fatalf("build: %v", err)
    }
    if len(file.Scripts) != 1 {
        t.Fatalf("expected 1 script, got %d", len(file.Scripts))
    }
    s := file.Scripts[0]
    if s.Name != "Player" {
        t.Errorf("name: got %q, want %q", s.Name, "Player")
    }
    if len(s.Fields) != 1 || s.Fields[0].Modifier != FieldInspect {
        t.Errorf("fields: %+v", s.Fields)
    }
    if len(s.Handlers) != 1 || s.Handlers[0].Kind != HandlerUpdate {
        t.Errorf("handlers: %+v", s.Handlers)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement `BuildAST`**

Create `ast/builder.go`. Walk the gotreesitter CST node tree, matching node types (`script_declaration`, `inspect_field`, `lifecycle_handler`, etc.) to AST constructors. Extract raw source text for bodies and expressions using byte range slicing on the input.

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Add builder tests for signals, reactive, ECS, passthrough**

Test each feature: signals parse to `Signal`/`Connect`, reactive fields parse to `Field` with `FieldReactive`/`FieldDerived` modifier, `watch_block` parses to `Watch`, `component_declaration` parses to `Component`, `system_declaration` with `query_block` parses to `System`/`Query`, raw Rust items parse to `RustItem`.

- [ ] **Step 6: Run all builder tests**

```bash
go test ./ast/ -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
buckley commit --yes --min
```

---

## Task 10: Script Transpiler

**Files:**
- Create: `transpiler/emit.go`
- Create: `transpiler/emit_test.go`
- Create: `transpiler/script.go`
- Create: `transpiler/script_test.go`

- [ ] **Step 1: Create Rust emitter utility**

Create `transpiler/emit.go` — a `RustEmitter` struct with methods: `Line(s)`, `Blank()`, `Block(header, fn)` (emits `header {` + indented body + `}`), `Derive(traits...)`, `Attribute(attr)`. Manages indentation and import collection. Has `String() string` to produce final output.

- [ ] **Step 2: Write test for emitter**

Create `transpiler/emit_test.go`:

```go
func TestEmitterBasic(t *testing.T) {
    e := NewEmitter()
    e.Derive("Debug", "Clone")
    e.Line("pub struct Foo {")
    e.Indent()
    e.Line("pub x: f32,")
    e.Dedent()
    e.Line("}")
    out := e.String()
    if !strings.Contains(out, "#[derive(Debug, Clone)]") {
        t.Errorf("missing derive: %s", out)
    }
}
```

- [ ] **Step 3: Run test**

- [ ] **Step 4: Write failing test for script transpilation**

Create `transpiler/script_test.go`:

```go
func TestTranspileMinimalScript(t *testing.T) {
    s := ast.Script{
        Name: "Player",
        Fields: []ast.Field{
            {Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Default: "10.0"},
        },
        Handlers: []ast.Handler{
            {Kind: ast.HandlerUpdate, Params: []ast.Param{{Name: "ctx"}}, Body: "self.speed += 1.0;"},
        },
    }
    out := TranspileScript(s)
    // Verify key markers in output
    if !strings.Contains(out, "derive(Visit, Reflect") {
        t.Errorf("missing derives: %s", out)
    }
    if !strings.Contains(out, "pub speed: f32") {
        t.Errorf("missing field: %s", out)
    }
    if !strings.Contains(out, "impl ScriptTrait for Player") {
        t.Errorf("missing ScriptTrait impl: %s", out)
    }
    if !strings.Contains(out, "fn on_update") {
        t.Errorf("missing on_update: %s", out)
    }
}
```

- [ ] **Step 5: Implement `TranspileScript`**

Create `transpiler/script.go`. Takes `ast.Script`, emits:
1. Struct with correct derives and UUID attribute
2. Fields with `pub` / `#[reflect(expand)]` / `#[reflect(hidden)]` / `#[visit(skip)]` based on modifier
3. `Default` impl with field initializers
4. `ScriptTrait` impl with handler bodies pasted into the right methods

- [ ] **Step 6: Run test to verify it passes**

- [ ] **Step 7: Commit**

```bash
buckley commit --yes --min
```

---

## Task 11: Node/Resource Field Transpiler

**Files:**
- Create: `transpiler/nodes.go`
- Create: `transpiler/nodes_test.go`

- [ ] **Step 1: Write failing test for node field resolution**

```go
func TestTranspileNodeField(t *testing.T) {
    s := ast.Script{
        Name: "Door",
        Fields: []ast.Field{
            {Modifier: ast.FieldNode, Name: "mesh", TypeExpr: "Mesh", Default: `"DoorMesh"`},
            {Modifier: ast.FieldResource, Name: "sound", TypeExpr: "SoundBuffer", Default: `"res://audio/creak.wav"`},
        },
    }
    out := TranspileScript(s)
    // node field becomes Handle<Node>
    if !strings.Contains(out, "mesh: Handle<Node>") {
        t.Errorf("missing Handle<Node>: %s", out)
    }
    // on_start should resolve node path
    if !strings.Contains(out, "fn on_start") {
        t.Errorf("missing on_start for node resolution: %s", out)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement node/resource field codegen**

In `transpiler/nodes.go`, add logic to `TranspileScript`:
- `node` fields emit `Handle<Node>` with resolution code in a generated `on_start`
- `nodes` (wildcard) fields emit `Vec<Handle<Node>>` with child enumeration
- `resource` fields emit resource handle type with load in `on_start`
- Generated `on_start` is merged with any user-defined `on start` handler

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 12: Handler Body Rewriting (Self-Node Shortcuts + Spawn)

**Files:**
- Create: `transpiler/body.go`
- Create: `transpiler/body_test.go`

Handler bodies are mostly raw Rust, but FyroxScript adds shortcuts that need rewriting before emit.

- [ ] **Step 1: Write failing test for self-node shortcuts**

Create `transpiler/body_test.go`:

```go
func TestRewriteSelfShortcuts(t *testing.T) {
    body := `let pos = self.position();
let fwd = self.forward();
let p = self.parent();
self.node.rotate_y(0.5);`

    out := RewriteBody(body, "MyScript")
    if !strings.Contains(out, "ctx.scene.graph[ctx.handle].global_position()") {
        t.Errorf("self.position() not rewritten: %s", out)
    }
    if !strings.Contains(out, "ctx.scene.graph[ctx.handle].look_direction()") {
        t.Errorf("self.forward() not rewritten: %s", out)
    }
    if !strings.Contains(out, "ctx.scene.graph[ctx.handle].parent()") {
        t.Errorf("self.parent() not rewritten: %s", out)
    }
    if !strings.Contains(out, "ctx.scene.graph[ctx.handle].rotate_y(0.5)") {
        t.Errorf("self.node not rewritten: %s", out)
    }
}
```

- [ ] **Step 2: Write failing test for spawn...at**

```go
func TestRewriteSpawn(t *testing.T) {
    body := `let goblin = spawn self.prefab at Vector3::new(0.0, 1.0, 0.0);`
    out := RewriteBody(body, "Spawner")
    if !strings.Contains(out, "instantiate") {
        t.Errorf("spawn not rewritten: %s", out)
    }
    if !strings.Contains(out, "set_position") {
        t.Errorf("at position not rewritten: %s", out)
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

- [ ] **Step 4: Implement `RewriteBody`**

In `transpiler/body.go`:
- `self.position()` → `ctx.scene.graph[ctx.handle].global_position()`
- `self.forward()` → `ctx.scene.graph[ctx.handle].look_direction()`
- `self.parent()` → `ctx.scene.graph[ctx.handle].parent()`
- `self.node.X` → `ctx.scene.graph[ctx.handle].X`
- `spawn EXPR at POS` → resource instantiation + set_position
- All other content passes through unchanged

Use string-based rewriting (not full AST transform) — these are well-defined textual patterns.

- [ ] **Step 5: Run tests to verify they pass**

- [ ] **Step 6: Commit**

```bash
buckley commit --yes --min
```

---

## Task 13: Event Signals Transpiler

**Files:**
- Create: `transpiler/signals.go`
- Create: `transpiler/signals_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestTranspileSignals(t *testing.T) {
    file := ast.File{
        Scripts: []ast.Script{
            {
                Name: "Enemy",
                Signals: []ast.Signal{
                    {Name: "died", Params: []ast.Param{{Name: "position", TypeExpr: "Vector3"}}},
                },
            },
            {
                Name: "Tracker",
                Connects: []ast.Connect{
                    {ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Body: "self.score += 1;"},
                },
            },
        },
    }
    out := TranspileSignals(file.Scripts)
    // Signal becomes a message struct
    if !strings.Contains(out, "struct EnemyDiedMsg") {
        t.Errorf("missing message struct: %s", out)
    }
    // Connect generates subscribe_to in on_start
    if !strings.Contains(out, "subscribe_to::<EnemyDiedMsg>") {
        t.Errorf("missing subscribe_to: %s", out)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement signal transpilation**

In `transpiler/signals.go`:
- `signal died(position: Vector3)` → `#[derive(Debug, Clone)] struct EnemyDiedMsg { pub position: Vector3 }`
- `emit died(pos)` → `ctx.message_sender.send_global(EnemyDiedMsg { position: pos })`
- `emit ... to target` → `ctx.message_sender.send_to_target(target, ...)`
- `connect Enemy::died(pos) { ... }` → `subscribe_to::<EnemyDiedMsg>` in `on_start` + dispatch in `on_message`

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 14: Reactive Signals Transpiler

**Files:**
- Create: `transpiler/reactive.go`
- Create: `transpiler/reactive_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestTranspileReactive(t *testing.T) {
    s := ast.Script{
        Name: "HUD",
        Fields: []ast.Field{
            {Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
            {Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health < 20.0"},
        },
        Watches: []ast.Watch{
            {Field: "self.is_critical", Body: "do_thing();"},
        },
    }
    out := TranspileScript(s)
    // Reactive generates shadow _prev field
    if !strings.Contains(out, "_health_prev: f32") {
        t.Errorf("missing shadow field: %s", out)
    }
    // Derived recomputation in on_update
    if !strings.Contains(out, "self.is_critical = self.health < 20.0") {
        t.Errorf("missing derived recompute: %s", out)
    }
    // Watch conditional
    if !strings.Contains(out, "_is_critical_prev") {
        t.Errorf("missing watch dirty-check: %s", out)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement reactive transpilation**

In `transpiler/reactive.go`:
- `reactive` fields add a shadow `_prev` field of the same type
- `derived` fields add recomputation at the top of generated `on_update`
- `watch` blocks add conditional execution after derived recomputation: `if self.field != self._field_prev { body; self._field_prev = self.field.clone(); }`
- All reactive code is prepended to the user's `on update` body

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 15: ECS Transpiler

**Files:**
- Create: `transpiler/ecs.go`
- Create: `transpiler/ecs_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestTranspileECS(t *testing.T) {
    file := ast.File{
        Components: []ast.Component{
            {Name: "Velocity", Fields: []ast.Field{
                {Modifier: ast.FieldBare, Name: "linear", TypeExpr: "Vector3"},
            }},
        },
        Systems: []ast.System{
            {
                Name: "move_things",
                Params: []ast.Param{{Name: "dt", TypeExpr: "f32"}},
                Queries: []ast.Query{
                    {
                        Params: []ast.QueryParam{
                            {Name: "pos", Mutable: true, TypeExpr: "Transform"},
                            {Name: "vel", Mutable: false, TypeExpr: "Velocity"},
                        },
                        Body: "pos.translate(vel.linear * dt);",
                    },
                },
            },
        },
    }
    out := TranspileECS(file.Components, file.Systems)
    if !strings.Contains(out, "struct Velocity") {
        t.Errorf("missing component struct: %s", out)
    }
    if !strings.Contains(out, "fn system_move_things") {
        t.Errorf("missing system function: %s", out)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement ECS transpilation**

In `transpiler/ecs.go`:
- `component` → plain Rust struct with `#[derive(Clone)]`
- `system` → a free function taking `&mut EcsWorld` and `&PluginContext`
- `query` → typed iteration: `for (entity, (pos, vel)) in world.query_mut::<(&mut Transform, &Velocity)>()`
- `ecs.spawn(...)` → `world.spawn((Component1 {...}, Component2 {...}))`
- `despawn(entity)` → `world.despawn(entity)`
- System invocations generated in a `pub fn run_ecs_systems(world, ctx)` function

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 16: Rust ECS Runtime Crate

**Files:**
- Create: `runtime/Cargo.toml`
- Create: `runtime/src/lib.rs`
- Create: `runtime/src/ecs.rs`

- [ ] **Step 1: Create Cargo.toml**

```toml
[package]
name = "fyroxscript-runtime"
version = "0.1.0"
edition = "2021"

[dependencies]
```

No external dependencies — the ECS is self-contained.

- [ ] **Step 2: Implement sparse-set ECS storage**

Create `runtime/src/ecs.rs`:
- `Entity` type (u64 ID)
- `EcsWorld` struct with `spawn`, `despawn`, `query`, `query_mut` methods
- Sparse-set component storage keyed by `TypeId`
- Generic `query<T: ComponentBundle>` returning typed iterators

Keep it minimal — this supports the transpiled system functions, not a general-purpose ECS.

- [ ] **Step 3: Write Rust tests**

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Clone)]
    struct Position { x: f32, y: f32 }

    #[derive(Clone)]
    struct Velocity { dx: f32, dy: f32 }

    #[test]
    fn spawn_and_query() {
        let mut world = EcsWorld::new();
        world.spawn((Position { x: 0.0, y: 0.0 }, Velocity { dx: 1.0, dy: 0.0 }));
        let count = world.query::<(&Position, &Velocity)>().count();
        assert_eq!(count, 1);
    }

    #[test]
    fn despawn() {
        let mut world = EcsWorld::new();
        let e = world.spawn((Position { x: 0.0, y: 0.0 },));
        world.despawn(e);
        assert_eq!(world.query::<(&Position,)>().count(), 0);
    }
}
```

- [ ] **Step 4: Run Rust tests**

```bash
cd runtime && cargo test
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 17: File-Level Transpiler + Plugin Registration

**Files:**
- Create: `transpiler/file.go`
- Create: `transpiler/file_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestTranspileFile(t *testing.T) {
    file := ast.File{
        Scripts: []ast.Script{
            {Name: "Player", Fields: []ast.Field{{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Default: "10.0"}}},
            {Name: "Enemy", Fields: []ast.Field{{Modifier: ast.FieldBare, Name: "health", TypeExpr: "f32"}}},
        },
        RustItems: []ast.RustItem{
            {Source: "use fyrox::prelude::*;"},
        },
    }
    out := TranspileFile(file)
    // Should have use statement at top
    if !strings.Contains(out, "use fyrox::prelude::*;") {
        t.Errorf("missing use statement: %s", out)
    }
    // Should have both scripts
    if !strings.Contains(out, "struct Player") || !strings.Contains(out, "struct Enemy") {
        t.Errorf("missing scripts: %s", out)
    }
}
```

- [ ] **Step 2: Implement `TranspileFile`**

In `transpiler/file.go`:
- Emit Rust items (passthrough) first
- Emit signal message structs
- Emit component structs
- Emit script structs + ScriptTrait impls
- Emit system functions
- Emit `pub fn register_scripts(ctx: &mut PluginRegistrationContext)` with `.add::<T>("Name")` for each script
- Emit `pub fn run_ecs_systems(world: &mut EcsWorld, ctx: &mut PluginContext)` calling each system in order

- [ ] **Step 3: Run test to verify it passes**

- [ ] **Step 4: Commit**

```bash
buckley commit --yes --min
```

---

## Task 18: CLI Build Tool

**Files:**
- Create: `cmd/fyxc/main.go`

- [ ] **Step 1: Implement `fyxc build`**

Create `cmd/fyxc/main.go`:
- `fyxc build [dir]` — finds all `.fyx` files in dir (default `game/src/`), parses each, transpiles, writes `.rs` files to `game/src/generated/`
- `fyxc build --check` — parse + type-check only, no output
- Generates a `mod.rs` in `generated/` that re-exports all generated modules
- Generates UUID `.fyx.meta` sidecars (deterministic from name + path)
- Prints summary: N scripts, N components, N systems transpiled

- [ ] **Step 2: Test with spec's complete example**

Create `testdata/full.fyx` with the weapon/HUD/projectile example from the spec. Run:

```bash
go run ./cmd/fyxc/ build testdata/
```

Verify generated `.rs` output compiles conceptually (manual inspection).

- [ ] **Step 3: Commit**

```bash
buckley commit --yes --min
```

---

## Task 19: Golden File Tests

**Files:**
- Create: `testdata/minimal.fyx`
- Create: `testdata/golden/minimal.rs`
- Create: `transpiler/golden_test.go`

- [ ] **Step 1: Create test fixtures**

Create `.fyx` files for each feature:
- `minimal.fyx` — single script with inspect field and on_update
- `signals.fyx` — signal/emit/connect across two scripts
- `reactive.fyx` — reactive/derived/watch
- `ecs.fyx` — component/system/query

- [ ] **Step 2: Generate golden `.rs` files**

Run `fyxc build testdata/` and manually verify each output. Copy verified output to `testdata/golden/`.

- [ ] **Step 3: Write golden comparison test**

Create `transpiler/golden_test.go`:

```go
func TestGoldenFiles(t *testing.T) {
    cases := []string{"minimal", "signals", "reactive", "ecs"}
    for _, name := range cases {
        t.Run(name, func(t *testing.T) {
            input, _ := os.ReadFile("../testdata/" + name + ".fyx")
            expected, _ := os.ReadFile("../testdata/golden/" + name + ".rs")
            lang := generateLang(t)
            file, err := ast.BuildAST(lang, input)
            if err != nil {
                t.Fatalf("build: %v", err)
            }
            got := TranspileFile(*file)
            if got != string(expected) {
                t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, string(expected))
            }
        })
    }
}
```

- [ ] **Step 4: Run golden tests**

```bash
go test ./transpiler/ -v -run TestGoldenFiles
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
buckley commit --yes --min
```

---

## Task 20: End-to-End Integration Test

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Write end-to-end test**

Create `integration_test.go` at the repo root:

```go
func TestEndToEndFullExample(t *testing.T) {
    // Parse the spec's complete example
    source, err := os.ReadFile("testdata/full.fyx")
    if err != nil {
        t.Fatalf("read: %v", err)
    }

    // Generate language
    g := grammar.FyroxScriptGrammar()
    lang, err := grammargen.GenerateLanguage(g)
    if err != nil {
        t.Fatalf("generate grammar: %v", err)
    }

    // Parse
    parser := gotreesitter.NewParser(lang)
    tree, err := parser.Parse(source)
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    sexpr := tree.RootNode().SExpr(lang)
    if strings.Contains(sexpr, "ERROR") {
        t.Fatalf("parse errors in full example: %s", sexpr)
    }

    // Build AST
    file, err := ast.BuildAST(lang, source)
    if err != nil {
        t.Fatalf("build AST: %v", err)
    }

    // Transpile
    out := transpiler.TranspileFile(*file)

    // Verify key markers
    markers := []string{
        "struct Weapon",
        "impl ScriptTrait for Weapon",
        "struct WeaponHUD",
        "struct Projectile",
        "struct Velocity",
        "fn system_move_projectiles",
        "fn system_expire_projectiles",
        "fn system_projectile_hits",
        "struct WeaponFiredMsg",
        "struct WeaponEmptiedMsg",
        "register_scripts",
        "run_ecs_systems",
    }
    for _, m := range markers {
        if !strings.Contains(out, m) {
            t.Errorf("missing %q in output:\n%s", m, out)
        }
    }
}
```

- [ ] **Step 2: Run test**

```bash
go test -v -run TestEndToEndFullExample -timeout 120s
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
buckley commit --yes --min
```
