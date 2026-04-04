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
	if len(extras) != 1 {
		t.Fatalf("expected 1 extra, got %d: %+v", len(extras), extras)
	}

	if extras[0].Name != "_health_prev" || extras[0].TypeExpr != "f32" {
		t.Errorf("extras[0] = %+v, want _health_prev: f32", extras[0])
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
	if len(inits) != 1 {
		t.Fatalf("expected 1 init line, got %d: %v", len(inits), inits)
	}

	if inits[0] != "_health_prev: 100.0," {
		t.Errorf("inits[0] = %q, want %q", inits[0], "_health_prev: 100.0,")
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

	// Reactive change tracking
	if !strings.Contains(code, "let _fyx_health_changed = self.health != self._health_prev;") {
		t.Errorf("missing reactive change flag: %s", code)
	}

	// Derived recomputation should be gated by reactive dependency changes
	if !strings.Contains(code, "let _fyx_is_critical_changed = if _fyx_health_changed {") {
		t.Errorf("missing gated derived recompute: %s", code)
	}
	if !strings.Contains(code, "self.is_critical = self.health < 20.0;") {
		t.Errorf("missing derived assignment: %s", code)
	}

	// Watch dirty-check uses the local changed flag
	if !strings.Contains(code, "if _fyx_is_critical_changed {") {
		t.Errorf("missing watch conditional: %s", code)
	}
	if !strings.Contains(code, "do_thing();") {
		t.Errorf("missing watch body: %s", code)
	}
	if strings.Contains(code, "self._is_critical_prev") {
		t.Errorf("derived fields should not keep struct-level shadow prev storage: %s", code)
	}

	// Reactive prev update is conditional
	if !strings.Contains(code, "if _fyx_health_changed {") || !strings.Contains(code, "self._health_prev = self.health.clone();") {
		t.Errorf("missing conditional reactive prev update: %s", code)
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
	if !strings.Contains(code, "let _fyx_speed_changed = self.speed != self._speed_prev;") {
		t.Errorf("missing reactive change flag: %s", code)
	}
	if !strings.Contains(code, "if _fyx_speed_changed {") || !strings.Contains(code, "self._speed_prev = self.speed.clone();") {
		t.Errorf("missing conditional reactive prev update: %s", code)
	}
	// No derived recomputation or watch blocks
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
	if !strings.Contains(code, "_fyx_health_changed") {
		t.Errorf("missing health change flag: %s", code)
	}
	if !strings.Contains(code, "_fyx_mana_changed") {
		t.Errorf("missing mana change flag: %s", code)
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
	if !strings.Contains(code, "_fyx_is_critical_changed") {
		t.Errorf("missing derived change tracking: %s", code)
	}
	if !strings.Contains(code, "_health_prev") {
		t.Errorf("missing reactive prev update: %s", code)
	}
}

func TestGenerateReactiveUpdateCodeDerivedChain(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldDerived, Name: "health_pct", TypeExpr: "f32", Default: "self.health / 100.0"},
		{Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health_pct < 0.2"},
	}

	code := GenerateReactiveUpdateCode(fields, nil)
	if !strings.Contains(code, "let _fyx_health_pct_changed = if _fyx_health_changed {") {
		t.Errorf("first derived field should be gated by reactive dependency changes: %s", code)
	}
	if !strings.Contains(code, "let _fyx_is_critical_changed = if _fyx_health_pct_changed {") {
		t.Errorf("derived chains should gate on upstream derived change flags: %s", code)
	}
}

func TestGenerateReactiveUpdateCodeDerivedChainOutOfOrder(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health_pct < 0.2"},
		{Modifier: ast.FieldDerived, Name: "health_pct", TypeExpr: "f32", Default: "self.health / 100.0"},
	}

	code := GenerateReactiveUpdateCode(fields, nil)
	pctIdx := strings.Index(code, "let _fyx_health_pct_changed")
	criticalIdx := strings.Index(code, "let _fyx_is_critical_changed")
	if pctIdx < 0 || criticalIdx < 0 {
		t.Fatalf("missing derived change blocks: %s", code)
	}
	if pctIdx > criticalIdx {
		t.Fatalf("derived recompute order should follow dependencies, got:\n%s", code)
	}
	if !strings.Contains(code, "let _fyx_is_critical_changed = if _fyx_health_pct_changed {") {
		t.Fatalf("out-of-order derived field should still gate on upstream derived change flag: %s", code)
	}
}

func TestGenerateReactiveUpdateCodeFallsBackToUngatedDerivedWhenDepsAreNotTracked(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldInspect, Name: "max_health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
		{Modifier: ast.FieldDerived, Name: "health_pct", TypeExpr: "f32", Default: "self.health / self.max_health"},
	}

	code := GenerateReactiveUpdateCode(fields, nil)
	if !strings.Contains(code, "let _fyx_health_pct_changed = {") {
		t.Errorf("derived fields with non-reactive dependencies should use ungated recompute blocks: %s", code)
	}
	if strings.Contains(code, "let _fyx_health_pct_changed = if") {
		t.Errorf("derived fields with inspect dependencies should not gate on incomplete change tracking: %s", code)
	}
}

func TestTranspileScriptDerivedDefaultsFollowDependencyOrder(t *testing.T) {
	s := ast.Script{
		Name: "Vitals",
		Fields: []ast.Field{
			{Modifier: ast.FieldReactive, Name: "health", TypeExpr: "f32", Default: "100.0"},
			{Modifier: ast.FieldDerived, Name: "is_critical", TypeExpr: "bool", Default: "self.health_pct < 0.2"},
			{Modifier: ast.FieldDerived, Name: "health_pct", TypeExpr: "f32", Default: "self.health / 100.0"},
		},
	}

	out := TranspileScript(s)
	pctIdx := strings.Index(out, "value.health_pct = value.health / 100.0;")
	criticalIdx := strings.Index(out, "value.is_critical = value.health_pct < 0.2;")
	if pctIdx < 0 || criticalIdx < 0 {
		t.Fatalf("missing derived default initialization lines: %s", out)
	}
	if pctIdx > criticalIdx {
		t.Fatalf("derived default initialization should follow dependencies: %s", out)
	}
}
