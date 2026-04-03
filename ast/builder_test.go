package ast

import (
	"testing"

	"github.com/odvcencio/fyx/grammar"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

func lang(t *testing.T) *gotreesitter.Language {
	t.Helper()
	g := grammar.FyxGrammar()
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
	if s.Fields[0].Name != "speed" || s.Fields[0].TypeExpr != "f32" || s.Fields[0].Default != "10.0" {
		t.Errorf("field details: %+v", s.Fields[0])
	}
	if len(s.Handlers) != 1 || s.Handlers[0].Kind != HandlerUpdate {
		t.Errorf("handlers: %+v", s.Handlers)
	}
}

func TestBuildSignals(t *testing.T) {
	l := lang(t)
	source := []byte(`script Enemy {
    signal died(position: Vector3)

    connect Other::hit(pos) {
        do_thing();
    }
}`)
	file, err := BuildAST(l, source)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := file.Scripts[0]
	if len(s.Signals) != 1 || s.Signals[0].Name != "died" {
		t.Errorf("signals: %+v", s.Signals)
	}
	if len(s.Connects) != 1 || s.Connects[0].ScriptName != "Other" || s.Connects[0].SignalName != "hit" {
		t.Errorf("connects: %+v", s.Connects)
	}
}

func TestBuildReactive(t *testing.T) {
	l := lang(t)
	source := []byte(`script HUD {
    reactive health: f32 = 100.0
    derived is_low: bool = self.health < 20.0

    watch self.is_low {
        alert();
    }
}`)
	file, err := BuildAST(l, source)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := file.Scripts[0]
	if len(s.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(s.Fields))
	}
	if s.Fields[0].Modifier != FieldReactive || s.Fields[1].Modifier != FieldDerived {
		t.Errorf("field modifiers: %+v, %+v", s.Fields[0], s.Fields[1])
	}
	if len(s.Watches) != 1 || s.Watches[0].Field != "self.is_low" {
		t.Errorf("watches: %+v", s.Watches)
	}
}

func TestBuildECS(t *testing.T) {
	l := lang(t)
	source := []byte(`component Velocity {
    linear: Vector3
}

system move(dt: f32) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt);
    }
}`)
	file, err := BuildAST(l, source)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(file.Components) != 1 || file.Components[0].Name != "Velocity" {
		t.Errorf("components: %+v", file.Components)
	}
	if len(file.Systems) != 1 || file.Systems[0].Name != "move" {
		t.Errorf("systems: %+v", file.Systems)
	}
	if len(file.Systems[0].Queries) != 1 {
		t.Errorf("queries: %+v", file.Systems[0].Queries)
	}
	q := file.Systems[0].Queries[0]
	if len(q.Params) != 2 || q.Params[0].Name != "pos" || !q.Params[0].Mutable {
		t.Errorf("query params: %+v", q.Params)
	}
}

func TestBuildRustPassthrough(t *testing.T) {
	l := lang(t)
	source := []byte(`use fyrox::prelude::*;

script Foo {
}

fn helper() -> i32 {
    42
}`)
	file, err := BuildAST(l, source)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(file.Scripts) != 1 {
		t.Errorf("expected 1 script, got %d", len(file.Scripts))
	}
	if len(file.RustItems) != 2 {
		t.Errorf("expected 2 rust items, got %d", len(file.RustItems))
	}
}

func TestBuildImports(t *testing.T) {
	l := lang(t)
	source := []byte(`import combat.weapon
import ui::hud

script Player {}
`)

	file, err := BuildAST(l, source)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(file.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(file.Imports))
	}
	if file.Imports[0].Path != "combat.weapon" {
		t.Fatalf("unexpected first import: %+v", file.Imports[0])
	}
	if file.Imports[1].Path != "ui.hud" {
		t.Fatalf("unexpected second import: %+v", file.Imports[1])
	}
}

func TestBuildArbiterDecls(t *testing.T) {
	l := lang(t)
	source := []byte(`source npc_senses {
    path sensor://vision
}

worker decide_directive {
    input ThreatOutcome
    output NpcDirective
}

rule PlayerDetected {
    when distance_to_player < 8.0
}

arbiter npc_brain {
    poll every_frame
    use_worker decide_directive
}
`)

	file, err := BuildAST(l, source)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(file.ArbiterDecls) != 4 {
		t.Fatalf("expected 4 arbiter declarations, got %d", len(file.ArbiterDecls))
	}
	if file.ArbiterDecls[0].Kind != ArbiterDeclSource || file.ArbiterDecls[0].Name != "npc_senses" {
		t.Fatalf("unexpected source declaration: %+v", file.ArbiterDecls[0])
	}
	if file.ArbiterDecls[1].Kind != ArbiterDeclWorker || file.ArbiterDecls[1].Name != "decide_directive" {
		t.Fatalf("unexpected worker declaration: %+v", file.ArbiterDecls[1])
	}
	if file.ArbiterDecls[2].Kind != ArbiterDeclRule || file.ArbiterDecls[2].Name != "PlayerDetected" {
		t.Fatalf("unexpected rule declaration: %+v", file.ArbiterDecls[2])
	}
	if file.ArbiterDecls[3].Kind != ArbiterDeclArbiter || file.ArbiterDecls[3].Name != "npc_brain" {
		t.Fatalf("unexpected arbiter declaration: %+v", file.ArbiterDecls[3])
	}
}
