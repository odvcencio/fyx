package grammar

import (
	"strings"
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
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
	if !strings.Contains(sexpr, "rust_item") {
		t.Errorf("expected rust_item nodes for use/fn/struct/impl, got: %s", sexpr)
	}
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
