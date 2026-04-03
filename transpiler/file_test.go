package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyrox-lang/ast"
)

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
	if !strings.Contains(out, "use fyrox::prelude::*;") {
		t.Errorf("missing use statement: %s", out)
	}
	if !strings.Contains(out, "struct Player") {
		t.Errorf("missing Player struct: %s", out)
	}
	if !strings.Contains(out, "struct Enemy") {
		t.Errorf("missing Enemy struct: %s", out)
	}
	if !strings.Contains(out, "register_scripts") {
		t.Errorf("missing register_scripts: %s", out)
	}
	if !strings.Contains(out, `add::<Player>("Player")`) {
		t.Errorf("missing Player registration: %s", out)
	}
}

func TestTranspileFileWithSignals(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Enemy",
				Signals: []ast.Signal{
					{Name: "died", Params: []ast.Param{{Name: "pos", TypeExpr: "Vector3"}}},
				},
			},
		},
	}
	out := TranspileFile(file)
	if !strings.Contains(out, "EnemyDiedMsg") {
		t.Errorf("missing signal struct: %s", out)
	}
}

func TestTranspileFileWithECS(t *testing.T) {
	file := ast.File{
		Components: []ast.Component{
			{Name: "Vel", Fields: []ast.Field{{Modifier: ast.FieldBare, Name: "x", TypeExpr: "f32"}}},
		},
		Systems: []ast.System{
			{Name: "tick", Queries: []ast.Query{{
				Params: []ast.QueryParam{{Name: "v", Mutable: true, TypeExpr: "Vel"}},
				Body:   "v.x += 1.0;",
			}}},
		},
	}
	out := TranspileFile(file)
	if !strings.Contains(out, "struct Vel") {
		t.Errorf("missing component: %s", out)
	}
	if !strings.Contains(out, "run_ecs_systems") {
		t.Errorf("missing system runner: %s", out)
	}
}

func TestTranspileFileOrdering(t *testing.T) {
	file := ast.File{
		RustItems: []ast.RustItem{
			{Source: "use fyrox::prelude::*;"},
		},
		Scripts: []ast.Script{
			{
				Name: "Player",
				Fields: []ast.Field{
					{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32"},
				},
				Signals: []ast.Signal{
					{Name: "scored", Params: []ast.Param{{Name: "points", TypeExpr: "i32"}}},
				},
			},
		},
		Components: []ast.Component{
			{Name: "Pos", Fields: []ast.Field{{Modifier: ast.FieldBare, Name: "x", TypeExpr: "f32"}}},
		},
		Systems: []ast.System{
			{Name: "move_sys", Queries: []ast.Query{{
				Params: []ast.QueryParam{{Name: "p", Mutable: true, TypeExpr: "Pos"}},
				Body:   "p.x += 1.0;",
			}}},
		},
	}
	out := TranspileFile(file)

	// Verify ordering: use < signal structs < components < script struct < systems < register
	useIdx := strings.Index(out, "use fyrox::prelude::*;")
	signalIdx := strings.Index(out, "PlayerScoredMsg")
	componentIdx := strings.Index(out, "struct Pos")
	scriptIdx := strings.Index(out, "pub struct Player {")
	systemIdx := strings.Index(out, "system_move_sys")
	registerIdx := strings.Index(out, "register_scripts")

	if useIdx < 0 {
		t.Fatal("missing use item")
	}
	if signalIdx < 0 {
		t.Fatal("missing signal struct")
	}
	if componentIdx < 0 {
		t.Fatal("missing component struct")
	}
	if scriptIdx < 0 {
		t.Fatal("missing script struct (pub struct Player)")
	}
	if systemIdx < 0 {
		t.Fatal("missing system function")
	}
	if registerIdx < 0 {
		t.Fatal("missing register_scripts")
	}

	if useIdx >= signalIdx {
		t.Error("use should come before signal structs")
	}
	if signalIdx >= componentIdx {
		t.Error("signal structs should come before components")
	}
	if componentIdx >= scriptIdx {
		t.Error("components should come before script structs")
	}
	if scriptIdx >= systemIdx {
		t.Error("script structs should come before systems")
	}
	if systemIdx >= registerIdx {
		t.Error("systems should come before register_scripts")
	}
}

func TestTranspileFileReactiveIntegration(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Health",
				Fields: []ast.Field{
					{Modifier: ast.FieldReactive, Name: "hp", TypeExpr: "f32", Default: "100.0"},
				},
				Watches: []ast.Watch{
					{Field: "self.hp", Body: "log::info!(\"hp changed\");"},
				},
			},
		},
	}
	out := TranspileFile(file)

	// Shadow field should appear in struct
	if !strings.Contains(out, "_hp_prev") {
		t.Errorf("missing reactive shadow field: %s", out)
	}
	// Reactive update code should appear in on_update
	if !strings.Contains(out, "self.hp != self._hp_prev") {
		t.Errorf("missing reactive watch conditional: %s", out)
	}
}

func TestTranspileFileConnectIntegration(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "ScoreBoard",
				Connects: []ast.Connect{
					{
						ScriptName: "Enemy",
						SignalName: "died",
						Params:     []string{"pos"},
						Body:       "self.score += 100;",
					},
				},
			},
		},
	}
	out := TranspileFile(file)

	// Signal subscription in on_start
	if !strings.Contains(out, "subscribe_to::<EnemyDiedMsg>") {
		t.Errorf("missing signal subscription: %s", out)
	}
	// Signal dispatch in on_message
	if !strings.Contains(out, "downcast_ref::<EnemyDiedMsg>") {
		t.Errorf("missing signal dispatch: %s", out)
	}
}

func TestTranspileFileEmitRewrite(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Enemy",
				Signals: []ast.Signal{
					{Name: "died", Params: []ast.Param{{Name: "position", TypeExpr: "Vector3"}}},
				},
				Handlers: []ast.Handler{
					{Kind: ast.HandlerUpdate, Body: "emit died(self.position());"},
				},
			},
		},
	}
	out := TranspileFile(file)

	if !strings.Contains(out, "send_global(EnemyDiedMsg") {
		t.Errorf("emit should be rewritten to send_global: %s", out)
	}
	if strings.Contains(out, "emit died") {
		t.Errorf("raw emit should not remain in output: %s", out)
	}
}

func TestTranspileFileEmpty(t *testing.T) {
	file := ast.File{}
	out := TranspileFile(file)
	// Empty file should still produce valid (albeit minimal) output
	out = strings.TrimSpace(out)
	if out != "" {
		t.Errorf("empty file should produce empty output, got: %s", out)
	}
}

func TestTranspileFileNoScriptsWithECS(t *testing.T) {
	file := ast.File{
		Components: []ast.Component{
			{Name: "Vel", Fields: []ast.Field{{Modifier: ast.FieldBare, Name: "x", TypeExpr: "f32"}}},
		},
		Systems: []ast.System{
			{Name: "tick", Queries: []ast.Query{{
				Params: []ast.QueryParam{{Name: "v", Mutable: true, TypeExpr: "Vel"}},
				Body:   "v.x += 1.0;",
			}}},
		},
	}
	out := TranspileFile(file)

	// No scripts means no register_scripts
	if strings.Contains(out, "register_scripts") {
		t.Errorf("should not have register_scripts with no scripts: %s", out)
	}
	if !strings.Contains(out, "struct Vel") {
		t.Errorf("missing component: %s", out)
	}
	if !strings.Contains(out, "run_ecs_systems") {
		t.Errorf("missing system runner: %s", out)
	}
}

func TestTranspileFileWithArbiterBundle(t *testing.T) {
	file := ast.File{
		ArbiterDecls: []ast.ArbiterDecl{
			{Kind: ast.ArbiterDeclWorker, Name: "decide_directive", Body: "input ThreatOutcome\noutput NpcDirective"},
			{Kind: ast.ArbiterDeclArbiter, Name: "npc_brain", Body: "poll 250ms\nsource sensor://vision"},
		},
	}
	out := TranspileFile(file)
	if !strings.Contains(out, "pub const FYX_ARBITER_BUNDLE") {
		t.Fatalf("missing arbiter bundle constant: %s", out)
	}
	if !strings.Contains(out, "worker decide_directive") || !strings.Contains(out, "arbiter npc_brain") {
		t.Fatalf("missing preserved arbiter source: %s", out)
	}
}

func TestTranspileFileMultipleRegistrations(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{Name: "Player"},
			{Name: "Enemy"},
			{Name: "Projectile"},
		},
	}
	out := TranspileFile(file)

	if !strings.Contains(out, `add::<Player>("Player")`) {
		t.Errorf("missing Player registration: %s", out)
	}
	if !strings.Contains(out, `add::<Enemy>("Enemy")`) {
		t.Errorf("missing Enemy registration: %s", out)
	}
	if !strings.Contains(out, `add::<Projectile>("Projectile")`) {
		t.Errorf("missing Projectile registration: %s", out)
	}
}
