package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
)

func TestRewriteSelfShortcuts(t *testing.T) {
	body := `let pos = self.position();
let fwd = self.forward();
let p = self.parent();
self.parent().script::<Weapon>();
self.node.rotate_y(0.5);`

	out := RewriteBody(body, "MyScript", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "ctx.scene.graph[ctx.handle].global_position()") {
		t.Errorf("self.position() not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[ctx.handle].look_vector()") {
		t.Errorf("self.forward() not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[ctx.handle].parent()") {
		t.Errorf("self.parent() not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[ctx.scene.graph[ctx.handle].parent()].script::<Weapon>()") {
		t.Errorf("self.parent().script::<T>() not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[ctx.handle].set_rotation_y(0.5)") {
		t.Errorf("self.node not rewritten: %s", out)
	}
}

func TestRewriteSpawn(t *testing.T) {
	body := `let goblin = spawn self.prefab at Vector3::new(0.0, 1.0, 0.0);`
	out := RewriteBody(body, "Spawner", nil, ast.HandlerUpdate)
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
	out := RewriteBody(body, "Test", nil, ast.HandlerUpdate)
	if out != body {
		t.Errorf("normal Rust should pass through unchanged, got: %s", out)
	}
}

func TestRewriteStandaloneNode(t *testing.T) {
	body := `let n = self.node;`
	out := RewriteBody(body, "Test", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "ctx.scene.graph[ctx.handle]") {
		t.Errorf("standalone self.node not rewritten: %s", out)
	}
}

func TestRewriteSpawnPosition(t *testing.T) {
	body := `spawn self.prefab at pos;`
	out := RewriteBody(body, "Test", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "let _resource = ctx.resource_manager.request::<Model>(self.prefab.clone());") {
		t.Errorf("spawn should request model paths through the resource manager: %s", out)
	}
	if !strings.Contains(out, "set_position(pos)") {
		t.Errorf("spawn at position not correctly extracted: %s", out)
	}
	if !strings.Contains(out, "self.prefab.clone()") {
		t.Errorf("spawn resource not correctly extracted: %s", out)
	}
}

func TestRewriteSpawnLoadedModelResourceField(t *testing.T) {
	body := `let goblin = spawn self.prefab at pos;`
	fields := []ast.Field{
		{Modifier: ast.FieldResource, Name: "prefab", TypeExpr: "Model"},
	}

	out := RewriteBody(body, "Spawner", fields, ast.HandlerUpdate)
	if !strings.Contains(out, `let _resource = self.prefab.clone().expect("Fyx resource field 'prefab' was not loaded before spawn");`) {
		t.Errorf("model resource fields should spawn from the loaded resource: %s", out)
	}
	if !strings.Contains(out, "_resource.instantiate(&mut ctx.scene.graph)") {
		t.Errorf("loaded model resource should instantiate directly: %s", out)
	}
	if strings.Contains(out, "request::<Model>(self.prefab.clone())") {
		t.Errorf("loaded model resource should not be re-requested: %s", out)
	}
}

func TestRewriteSpawnPreservesStatementBoundary(t *testing.T) {
	body := `let goblin = spawn self.prefab at pos;
let after = 1;`
	out := RewriteBody(body, "Spawner", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "_inst };\nlet after = 1;") {
		t.Errorf("spawn rewrite should preserve the original statement terminator: %s", out)
	}
}

func TestRewriteDoesNotRewriteSelfFields(t *testing.T) {
	body := `self.health -= 10.0;
self.name = "goblin".to_string();`
	out := RewriteBody(body, "Enemy", nil, ast.HandlerUpdate)
	if out != body {
		t.Errorf("self.field access should not be rewritten, got: %s", out)
	}
}

func TestRewriteNodeFieldMethods(t *testing.T) {
	body := `self.flash.set_visibility(false);
let muzzle = &self.muzzle.node();
let pos = self.muzzle.position();
let dir = self.muzzle.forward();
let parent = self.flash.parent();`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "flash"},
		{Modifier: ast.FieldNode, Name: "muzzle"},
	}

	out := RewriteBody(body, "Weapon", fields, ast.HandlerUpdate)
	if !strings.Contains(out, "ctx.scene.graph[self.flash].set_visibility(false)") {
		t.Errorf("node field method not rewritten: %s", out)
	}
	if !strings.Contains(out, "&ctx.scene.graph[self.muzzle]") {
		t.Errorf("node field node() shortcut not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[self.muzzle].global_position()") {
		t.Errorf("node field position shortcut not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[self.muzzle].look_vector()") {
		t.Errorf("node field forward shortcut not rewritten: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[self.flash].parent()") {
		t.Errorf("node field parent shortcut not rewritten: %s", out)
	}
}

func TestRewriteHandleVariableNodeAndScriptSugar(t *testing.T) {
	body := `let enemy = goblin.script::<Enemy>();
let enemy_node = goblin.node();`
	out := RewriteBody(body, "Spawner", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "let enemy = ctx.scene.graph[goblin].script::<Enemy>();") {
		t.Errorf("handle variable script::<T>() shortcut not rewritten: %s", out)
	}
	if !strings.Contains(out, "let enemy_node = ctx.scene.graph[goblin];") {
		t.Errorf("handle variable node() shortcut not rewritten: %s", out)
	}
}

func TestRewriteHandleAliasesFromNodeFieldsAndSpawn(t *testing.T) {
	body := `let muzzle = self.muzzle;
let origin = muzzle.position();
let parent = muzzle.parent();
if let Some(weapon) = parent.script::<Weapon>() {
    let _ = weapon;
}
let projectile = spawn self.prefab at origin;
let projectile_origin = projectile.position();
projectile.set_visibility(false);`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "muzzle"},
		{Modifier: ast.FieldResource, Name: "prefab", TypeExpr: "Model"},
	}

	out := RewriteBody(body, "Weapon", fields, ast.HandlerUpdate)
	if !strings.Contains(out, "let origin = ctx.scene.graph[muzzle].global_position();") {
		t.Errorf("node handle aliases should keep position() sugar: %s", out)
	}
	if !strings.Contains(out, "let parent = ctx.scene.graph[muzzle].parent();") {
		t.Errorf("node handle aliases should keep parent() sugar: %s", out)
	}
	if !strings.Contains(out, "if let Some(weapon) = ctx.scene.graph[parent].script::<Weapon>() {") {
		t.Errorf("parent handle aliases should keep script::<T>() sugar: %s", out)
	}
	if !strings.Contains(out, "let projectile_origin = ctx.scene.graph[projectile].global_position();") {
		t.Errorf("spawned handle aliases should keep position() sugar: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[projectile].set_visibility(false);") {
		t.Errorf("spawned handle aliases should keep node method sugar: %s", out)
	}
}

func TestRewriteDtShorthand(t *testing.T) {
	body := `self.cooldown -= dt;`
	out := RewriteBody(body, "Weapon", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "let dt = ctx.dt;") {
		t.Errorf("dt shorthand not injected: %s", out)
	}
	if !strings.Contains(out, "self.cooldown -= dt;") {
		t.Errorf("body should be preserved: %s", out)
	}
}

func TestRewriteScriptEcsSpawn(t *testing.T) {
	body := `let projectile = ecs.spawn(
    Projectile { damage: 25.0, lifetime: 1.0 },
    Velocity { linear: Vector3::default(), angular: Vector3::default() },
);`
	out := RewriteBody(body, "Spawner", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "ctx.ecs.spawn((Projectile { damage: 25.0, lifetime: 1.0 }, Velocity { linear: Vector3::default(), angular: Vector3::default() }))") {
		t.Errorf("ecs.spawn not rewritten to ctx.ecs bundle spawn: %s", out)
	}
}

func TestRewriteScriptEcsSpawnSingleComponent(t *testing.T) {
	body := `let marker = ecs.spawn(Marker { active: true });`
	out := RewriteBody(body, "Spawner", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "ctx.ecs.spawn((Marker { active: true },))") {
		t.Errorf("single-component ecs.spawn should become a single-element tuple: %s", out)
	}
}
