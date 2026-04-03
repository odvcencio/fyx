package transpiler

import (
	"strings"
	"testing"
)

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

func TestRewritePreservesNormalRust(t *testing.T) {
	body := `let x = 5;
self.speed += 1.0;
println!("hello");`
	out := RewriteBody(body, "Test")
	if out != body {
		t.Errorf("normal Rust should pass through unchanged, got: %s", out)
	}
}

func TestRewriteStandaloneNode(t *testing.T) {
	body := `let n = self.node;`
	out := RewriteBody(body, "Test")
	if !strings.Contains(out, "ctx.scene.graph[ctx.handle]") {
		t.Errorf("standalone self.node not rewritten: %s", out)
	}
}

func TestRewriteSpawnPosition(t *testing.T) {
	body := `spawn self.prefab at pos;`
	out := RewriteBody(body, "Test")
	if !strings.Contains(out, "set_position(pos)") {
		t.Errorf("spawn at position not correctly extracted: %s", out)
	}
	if !strings.Contains(out, "self.prefab.clone()") {
		t.Errorf("spawn resource not correctly extracted: %s", out)
	}
}

func TestRewriteDoesNotRewriteSelfFields(t *testing.T) {
	body := `self.health -= 10.0;
self.name = "goblin".to_string();`
	out := RewriteBody(body, "Enemy")
	if out != body {
		t.Errorf("self.field access should not be rewritten, got: %s", out)
	}
}
