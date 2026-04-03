package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyrox-lang/ast"
)

func TestTranspileMinimalScript(t *testing.T) {
	s := ast.Script{
		Name: "Player",
		Fields: []ast.Field{
			{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Default: "10.0"},
			{Modifier: ast.FieldBare, Name: "move_dir", TypeExpr: "Vector3"},
		},
		Handlers: []ast.Handler{
			{Kind: ast.HandlerUpdate, Params: []ast.Param{{Name: "ctx"}}, Body: "self.speed += 1.0;"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "#[derive(Visit, Reflect") {
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
	if !strings.Contains(out, "#[reflect(hidden)]") {
		t.Errorf("missing reflect hidden for bare field: %s", out)
	}
}

func TestTranspileScriptUUID(t *testing.T) {
	s := ast.Script{
		Name: "Player",
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "#[type_uuid(id = \"") {
		t.Errorf("missing type_uuid: %s", out)
	}
	// UUID should be deterministic
	out2 := TranspileScript(s)
	if out != out2 {
		t.Errorf("non-deterministic UUID generation")
	}
}

func TestTranspileScriptDefaultImpl(t *testing.T) {
	s := ast.Script{
		Name: "Enemy",
		Fields: []ast.Field{
			{Modifier: ast.FieldInspect, Name: "health", TypeExpr: "f32", Default: "100.0"},
			{Modifier: ast.FieldInspect, Name: "name", TypeExpr: "String", Default: `"goblin".to_string()`},
			{Modifier: ast.FieldBare, Name: "timer", TypeExpr: "f32"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "impl Default for Enemy") {
		t.Errorf("missing Default impl: %s", out)
	}
	if !strings.Contains(out, "health: 100.0") {
		t.Errorf("missing health default: %s", out)
	}
	if !strings.Contains(out, `name: "goblin".to_string()`) {
		t.Errorf("missing name default: %s", out)
	}
	if !strings.Contains(out, "..Default::default()") {
		t.Errorf("missing ..Default::default(): %s", out)
	}
}

func TestTranspileScriptNoDefaultImpl(t *testing.T) {
	s := ast.Script{
		Name: "Simple",
		Fields: []ast.Field{
			{Modifier: ast.FieldBare, Name: "x", TypeExpr: "f32"},
		},
	}
	out := TranspileScript(s)
	if strings.Contains(out, "impl Default for Simple") {
		t.Errorf("should not have Default impl when no defaults: %s", out)
	}
}

func TestTranspileScriptBareField(t *testing.T) {
	s := ast.Script{
		Name: "Foo",
		Fields: []ast.Field{
			{Modifier: ast.FieldBare, Name: "internal", TypeExpr: "i32"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "#[reflect(hidden)]") {
		t.Errorf("missing reflect hidden: %s", out)
	}
	if !strings.Contains(out, "#[visit(skip)]") {
		t.Errorf("missing visit skip: %s", out)
	}
	// Bare fields should NOT be pub
	if strings.Contains(out, "pub internal") {
		t.Errorf("bare field should not be pub: %s", out)
	}
}

func TestTranspileScriptInspectField(t *testing.T) {
	s := ast.Script{
		Name: "Bar",
		Fields: []ast.Field{
			{Modifier: ast.FieldInspect, Name: "visible", TypeExpr: "f64"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "#[reflect(expand)]") {
		t.Errorf("missing reflect expand: %s", out)
	}
	if !strings.Contains(out, "pub visible: f64") {
		t.Errorf("inspect field should be pub: %s", out)
	}
}

func TestTranspileScriptReactiveField(t *testing.T) {
	s := ast.Script{
		Name: "Baz",
		Fields: []ast.Field{
			{Modifier: ast.FieldReactive, Name: "score", TypeExpr: "i32"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "#[reflect(expand)]") {
		t.Errorf("reactive field should have reflect expand: %s", out)
	}
	if !strings.Contains(out, "pub score: i32") {
		t.Errorf("reactive field should be pub: %s", out)
	}
}

func TestTranspileScriptDerivedField(t *testing.T) {
	s := ast.Script{
		Name: "Computed",
		Fields: []ast.Field{
			{Modifier: ast.FieldDerived, Name: "total", TypeExpr: "f32"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "#[reflect(hidden)]") {
		t.Errorf("derived field should have reflect hidden: %s", out)
	}
	if !strings.Contains(out, "#[visit(skip)]") {
		t.Errorf("derived field should have visit skip: %s", out)
	}
	if strings.Contains(out, "pub total") {
		t.Errorf("derived field should not be pub: %s", out)
	}
}

func TestTranspileScriptNodeField(t *testing.T) {
	s := ast.Script{
		Name: "WithNode",
		Fields: []ast.Field{
			{Modifier: ast.FieldNode, Name: "camera", TypeExpr: "Camera", Default: `"MainCamera"`},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "pub camera: Handle<Node>") {
		t.Errorf("node field should be Handle<Node>: %s", out)
	}
	if !strings.Contains(out, "fn on_start") {
		t.Errorf("node field should generate on_start: %s", out)
	}
	if !strings.Contains(out, "find_by_name_from_root(\"MainCamera\")") {
		t.Errorf("missing find_by_name_from_root: %s", out)
	}
}

func TestTranspileScriptInitHandler(t *testing.T) {
	s := ast.Script{
		Name: "Startup",
		Handlers: []ast.Handler{
			{Kind: ast.HandlerInit, Params: []ast.Param{{Name: "ctx"}}, Body: "log::info!(\"init\");"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "fn on_init(&mut self, ctx: &mut ScriptContext)") {
		t.Errorf("missing on_init: %s", out)
	}
	if !strings.Contains(out, `log::info!("init");`) {
		t.Errorf("missing body: %s", out)
	}
}

func TestTranspileScriptDeinitHandler(t *testing.T) {
	s := ast.Script{
		Name: "Cleanup",
		Handlers: []ast.Handler{
			{Kind: ast.HandlerDeinit, Params: []ast.Param{{Name: "ctx"}}, Body: "cleanup();"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "fn on_deinit(&mut self, ctx: &mut ScriptDeinitContext)") {
		t.Errorf("missing on_deinit: %s", out)
	}
}

func TestTranspileScriptEventHandler(t *testing.T) {
	s := ast.Script{
		Name: "Input",
		Handlers: []ast.Handler{
			{
				Kind: ast.HandlerEvent,
				Params: []ast.Param{
					{Name: "ev", TypeExpr: "KeyboardInput"},
					{Name: "ctx"},
				},
				Body: "handle_key(ev);",
			},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "fn on_os_event(&mut self, event: &Event<()>, ctx: &mut ScriptContext)") {
		t.Errorf("missing on_os_event: %s", out)
	}
	if !strings.Contains(out, "WindowEvent::KeyboardInput(ev)") {
		t.Errorf("missing if-let match: %s", out)
	}
	if !strings.Contains(out, "handle_key(ev);") {
		t.Errorf("missing event body: %s", out)
	}
}

func TestTranspileScriptEventHandlerNoType(t *testing.T) {
	s := ast.Script{
		Name: "RawEvent",
		Handlers: []ast.Handler{
			{
				Kind:   ast.HandlerEvent,
				Params: []ast.Param{{Name: "ctx"}},
				Body:   "process(event);",
			},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "fn on_os_event") {
		t.Errorf("missing on_os_event: %s", out)
	}
	// Without a typed event param, body is emitted directly
	if !strings.Contains(out, "process(event);") {
		t.Errorf("missing body: %s", out)
	}
}

func TestTranspileScriptMultipleHandlers(t *testing.T) {
	s := ast.Script{
		Name: "Multi",
		Handlers: []ast.Handler{
			{Kind: ast.HandlerInit, Body: "init();"},
			{Kind: ast.HandlerUpdate, Body: "update();"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "fn on_init") {
		t.Errorf("missing on_init: %s", out)
	}
	if !strings.Contains(out, "fn on_update") {
		t.Errorf("missing on_update: %s", out)
	}
}

func TestTranspileScriptEmptyBody(t *testing.T) {
	s := ast.Script{
		Name: "Empty",
		Handlers: []ast.Handler{
			{Kind: ast.HandlerUpdate, Body: ""},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "fn on_update") {
		t.Errorf("missing on_update: %s", out)
	}
}

func TestTranspileScriptMultiLineBody(t *testing.T) {
	s := ast.Script{
		Name: "MultiLine",
		Handlers: []ast.Handler{
			{Kind: ast.HandlerUpdate, Body: "let x = 1;\nlet y = 2;\nx + y;"},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "let x = 1;") {
		t.Errorf("missing first line: %s", out)
	}
	if !strings.Contains(out, "let y = 2;") {
		t.Errorf("missing second line: %s", out)
	}
}

func TestScriptUUIDDeterministic(t *testing.T) {
	u1 := scriptUUID("Player")
	u2 := scriptUUID("Player")
	if u1 != u2 {
		t.Errorf("UUID not deterministic: %s != %s", u1, u2)
	}
	u3 := scriptUUID("Enemy")
	if u1 == u3 {
		t.Errorf("different scripts should have different UUIDs")
	}
}

func TestScriptUUIDFormat(t *testing.T) {
	uuid := scriptUUID("Test")
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Errorf("UUID should have 5 parts, got %d: %s", len(parts), uuid)
	}
	expectedLengths := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != expectedLengths[i] {
			t.Errorf("UUID part %d should be %d chars, got %d: %s", i, expectedLengths[i], len(p), uuid)
		}
	}
}
