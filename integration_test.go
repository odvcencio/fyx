package fyx_test

import (
	"os"
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/grammar"
	"github.com/odvcencio/fyx/transpiler"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

func TestEndToEndFullExample(t *testing.T) {
	// Read the full example fixture
	source, err := os.ReadFile("testdata/full.fyx")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Generate language
	g := grammar.FyxGrammar()
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
		t.Fatalf("parse errors in full example:\n%s", sexpr)
	}

	// Build AST
	file, err := ast.BuildAST(lang, source)
	if err != nil {
		t.Fatalf("build AST: %v", err)
	}

	// Verify AST structure
	if len(file.Scripts) != 3 {
		t.Errorf("expected 3 scripts, got %d", len(file.Scripts))
	}
	if len(file.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(file.Components))
	}
	if len(file.Systems) != 3 {
		t.Errorf("expected 3 systems, got %d", len(file.Systems))
	}

	// Transpile
	out := transpiler.TranspileFile(*file)

	// Verify key markers in output
	markers := []string{
		"struct Weapon",
		"impl ScriptTrait for Weapon",
		"struct WeaponHUD",
		"impl ScriptTrait for WeaponHUD",
		"struct Damageable",
		"impl ScriptTrait for Damageable",
		"struct Projectile",
		"struct Velocity",
		"fn system_move_projectiles",
		"fn system_expire_projectiles",
		"fn system_projectile_hits",
		"WeaponFiredMsg",
		"WeaponEmptiedMsg",
		"DamageableDamagedMsg",
		"ctx.scene.physics.raycast",
		"send_to_target(hit.node, DamageableDamagedMsg",
		"ctx.scene.graph[projectile].global_position()",
		"ctx.scene.graph[owner].global_position()",
		"register_scripts",
		"run_ecs_systems",
		"subscribe_to",
		"Handle<Node>",
	}
	for _, m := range markers {
		if !strings.Contains(out, m) {
			t.Errorf("missing %q in output:\n%s", m, out)
		}
	}

	// Verify no duplicate sections
	weaponCount := strings.Count(out, "pub struct Weapon {")
	if weaponCount != 1 {
		t.Errorf("expected exactly 1 Weapon struct, found %d", weaponCount)
	}

	t.Logf("Full transpilation output (%d bytes):\n%s", len(out), out)
}

func TestEndToEndDepthExample(t *testing.T) {
	source, err := os.ReadFile("testdata/depth.fyx")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	g := grammar.FyxGrammar()
	lang, err := grammargen.GenerateLanguage(g)
	if err != nil {
		t.Fatalf("generate grammar: %v", err)
	}

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sexpr := tree.RootNode().SExpr(lang)
	if strings.Contains(sexpr, "ERROR") {
		t.Fatalf("parse errors in depth example:\n%s", sexpr)
	}

	file, err := ast.BuildAST(lang, source)
	if err != nil {
		t.Fatalf("build AST: %v", err)
	}

	if len(file.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(file.Imports))
	}
	if len(file.RustItems) != 2 {
		t.Fatalf("expected 2 rust items, got %d", len(file.RustItems))
	}
	if len(file.Scripts) != 2 {
		t.Fatalf("expected 2 scripts, got %d", len(file.Scripts))
	}
	if len(file.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(file.Components))
	}
	if len(file.Systems) != 2 {
		t.Fatalf("expected 2 systems, got %d", len(file.Systems))
	}

	out := transpiler.TranspileFileResult(*file, transpiler.Options{
		CurrentModule: transpiler.ModulePathFromRelative("depth.fyx"),
		SignalIndex:   transpiler.BuildSignalIndex([]ast.File{*file}),
		SourcePath:    "depth.fyx",
	}).Code

	markers := []string{
		"use super::support::helpers::*;",
		"fn target_visible(scene: &Scene, origin: Vector3, direction: Vector3, range: f32) -> bool {",
		"pub struct TurretController {",
		"pub struct TurretHud {",
		"TurretControllerFiredMsg",
		"TurretControllerHeatChangedMsg",
		"ctx.message_dispatcher.subscribe_to::<TurretControllerFiredMsg>(ctx.handle);",
		"ctx.ecs.spawn((HeatTrail { heat: self.heat, ttl: 0.5 }, ShotOwner { node: self.muzzle }))",
		"fn on_os_event(&mut self, event: &Event<()>, ctx: &mut ScriptContext) -> GameResult {",
		"if let Event::WindowEvent { event: WindowEvent::MouseButton(button), .. } = event {",
		"ctx.scene.graph[self.pivot].set_rotation_y(self.turn_rate * 0.25);",
		"fn on_deinit(&mut self, ctx: &mut ScriptDeinitContext) -> GameResult {",
		"pub fn system_decay_heat_trails(world: &mut EcsWorld, ctx: &PluginContext) {",
		"pub fn system_inspect_heat_trails(world: &mut EcsWorld, ctx: &PluginContext) {",
	}
	for _, m := range markers {
		if !strings.Contains(out, m) {
			t.Errorf("missing %q in output:\n%s", m, out)
		}
	}
}
