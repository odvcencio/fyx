package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
)

func TestReactiveFieldDecls(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health < 20.0"},
		{Modifier: ast.FieldInspect, Name: "name", TypeExpr: "String", Default: "\"Player\".into()"},
	}

	extras := ReactiveFieldDecls(fields)
	if len(extras) != 2 {
		t.Fatalf("expected 2 extras, got %d: %+v", len(extras), extras)
	}

	if extras[0].Name != "_health_prev" || extras[0].TypeExpr != "f32" {
		t.Errorf("extras[0] = %+v, want _health_prev: f32", extras[0])
	}
	if extras[1].Name != "_is_critical_prev" || extras[1].TypeExpr != "bool" {
		t.Errorf("extras[1] = %+v, want _is_critical_prev: bool", extras[1])
	}
}

func TestReactiveFieldDeclsEmpty(t *testing.T) {
	extras := ReactiveFieldDecls(nil)
	if len(extras) != 0 {
		t.Errorf("expected 0 extras for nil fields, got %d", len(extras))
	}
}

func TestReactiveFieldDeclsNonReactive(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32"},
		{Modifier: ast.FieldBare, Name: "counter", TypeExpr: "i32"},
	}
	extras := ReactiveFieldDecls(fields)
	if len(extras) != 0 {
		t.Errorf("expected 0 extras for non-reactive fields, got %d: %+v", len(extras), extras)
	}
}

func TestReactiveDefaultInits(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health < 20.0"},
	}

	inits := ReactiveDefaultInits(fields)
	if len(inits) != 2 {
		t.Fatalf("expected 2 init lines, got %d: %v", len(inits), inits)
	}

	if inits[0] != "_health_prev: 100.0," {
		t.Errorf("inits[0] = %q, want %q", inits[0], "_health_prev: 100.0,")
	}
	if inits[1] != "_is_critical_prev: self.health < 20.0," {
		t.Errorf("inits[1] = %q, want %q", inits[1], "_is_critical_prev: self.health < 20.0,")
	}
}

func TestReactiveDefaultInitsNoDefault(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "score", TypeExpr: "i32"},
	}
	inits := ReactiveDefaultInits(fields)
	if len(inits) != 1 {
		t.Fatalf("expected 1 init line, got %d", len(inits))
	}
	if inits[0] != "_score_prev: Default::default()," {
		t.Errorf("inits[0] = %q, want %q", inits[0], "_score_prev: Default::default(),")
	}
}

func TestReactiveDefaultInitsEmpty(t *testing.T) {
	inits := ReactiveDefaultInits(nil)
	if len(inits) != 0 {
		t.Errorf("expected 0 init lines for nil fields, got %d", len(inits))
	}
}

func TestGenerateReactiveUpdateCode(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health < 20.0"},
	}
	watches := []ast.Watch{
		{Field: "self.is_critical", Body: "do_thing();"},
	}

	code := GenerateReactiveUpdateCode(fields, watches)

	// Derived recomputation
	if !strings.Contains(code, "self.is_critical = self.health < 20.0;") {
		t.Errorf("missing derived recompute: %s", code)
	}

	// Watch dirty-check
	if !strings.Contains(code, "if self.is_critical != self._is_critical_prev {") {
		t.Errorf("missing watch conditional: %s", code)
	}
	if !strings.Contains(code, "do_thing();") {
		t.Errorf("missing watch body: %s", code)
	}
	if !strings.Contains(code, "self._is_critical_prev = self.is_critical.clone();") {
		t.Errorf("missing watch prev update: %s", code)
	}

	// Reactive prev update
	if !strings.Contains(code, "self._health_prev = self.health.clone();") {
		t.Errorf("missing reactive prev update: %s", code)
	}
}

func TestGenerateReactiveUpdateCodeEmpty(t *testing.T) {
	code := GenerateReactiveUpdateCode(nil, nil)
	if code != "" {
		t.Errorf("expected empty code for nil fields/watches, got: %q", code)
	}
}

func TestGenerateReactiveUpdateCodeReactiveOnly(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "speed", TypeExpr: "f32", Default: "10.0"},
	}
	code := GenerateReactiveUpdateCode(fields, nil)
	if !strings.Contains(code, "self._speed_prev = self.speed.clone();") {
		t.Errorf("missing reactive prev update: %s", code)
	}
	// No derived recomputation or watch blocks
	if strings.Contains(code, "if ") {
		t.Errorf("unexpected watch conditional in reactive-only code: %s", code)
	}
}

func TestGenerateReactiveUpdateCodeMultipleWatches(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldReactive, Name: "mana", TypeExpr: "f32", Default: "50.0"},
	}
	watches := []ast.Watch{
		{Field: "self.health", Body: "update_health_bar();"},
		{Field: "self.mana", Body: "update_mana_bar();"},
	}
	code := GenerateReactiveUpdateCode(fields, watches)
	if !strings.Contains(code, "self._health_prev") {
		t.Errorf("missing _health_prev: %s", code)
	}
	if !strings.Contains(code, "self._mana_prev") {
		t.Errorf("missing _mana_prev: %s", code)
	}
	if !strings.Contains(code, "update_health_bar();") {
		t.Errorf("missing health watch body: %s", code)
	}
	if !strings.Contains(code, "update_mana_bar();") {
		t.Errorf("missing mana watch body: %s", code)
	}
}

// TestTranspileReactive is the integration test from the task spec.
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

	extras := ReactiveFieldDecls(s.Fields)
	found := false
	for _, ex := range extras {
		if ex.Name == "_health_prev" && ex.TypeExpr == "f32" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing _health_prev shadow field: %+v", extras)
	}

	code := GenerateReactiveUpdateCode(s.Fields, s.Watches)
	if !strings.Contains(code, "self.is_critical = self.health < 20.0") {
		t.Errorf("missing derived recompute: %s", code)
	}
	if !strings.Contains(code, "_is_critical_prev") {
		t.Errorf("missing watch dirty-check: %s", code)
	}
	if !strings.Contains(code, "_health_prev") {
		t.Errorf("missing reactive prev update: %s", code)
	}
}
