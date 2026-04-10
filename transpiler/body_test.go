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

func TestRewriteSpawnLoadedModelResourceAliases(t *testing.T) {
	body := `let prefab = self.prefab;
let active_prefab = prefab;
let goblin = spawn active_prefab at pos;`
	fields := []ast.Field{
		{Modifier: ast.FieldResource, Name: "prefab", TypeExpr: "Model"},
	}

	out := RewriteBody(body, "Spawner", fields, ast.HandlerUpdate)
	if !strings.Contains(out, `let _resource = active_prefab.clone().expect("Fyx resource field 'prefab' was not loaded before spawn");`) {
		t.Errorf("model resource aliases should spawn from the loaded resource: %s", out)
	}
	if strings.Contains(out, "request::<Model>(active_prefab.clone())") {
		t.Errorf("model resource aliases should not be re-requested: %s", out)
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

func TestRewriteNodesFieldIndexAliases(t *testing.T) {
	body := `let first_digit = self.ammo_digits[0];
let first_pos = self.ammo_digits[1].position();
let parent = first_digit.parent();
if let Some(weapon) = first_digit.script::<Weapon>() {
    let _ = (first_pos, parent, weapon);
}
first_digit.set_text("9");`
	fields := []ast.Field{
		{Modifier: ast.FieldNodes, Name: "ammo_digits"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, "let first_pos = ctx.scene.graph[self.ammo_digits[1]].global_position();") {
		t.Errorf("indexed nodes fields should expose position() sugar: %s", out)
	}
	if !strings.Contains(out, "let parent = ctx.scene.graph[first_digit].parent();") {
		t.Errorf("aliases from indexed nodes fields should expose parent() sugar: %s", out)
	}
	if !strings.Contains(out, "if let Some(weapon) = ctx.scene.graph[first_digit].script::<Weapon>() {") {
		t.Errorf("aliases from indexed nodes fields should expose script::<T>() sugar: %s", out)
	}
	if !strings.Contains(out, `ctx.scene.graph[first_digit].set_text("9");`) {
		t.Errorf("aliases from indexed nodes fields should expose node methods: %s", out)
	}
}

func TestRewriteNodesFieldLoopAliases(t *testing.T) {
	body := `for digit in self.ammo_digits {
    let parent = digit.parent();
    digit.set_text("9");
    let _ = parent;
}`
	fields := []ast.Field{
		{Modifier: ast.FieldNodes, Name: "ammo_digits"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, "let parent = ctx.scene.graph[digit].parent();") {
		t.Errorf("loop-bound nodes field handles should expose parent() sugar: %s", out)
	}
	if !strings.Contains(out, "for digit in self.ammo_digits.iter().cloned() {") {
		t.Errorf("bare nodes field loops should lower to cloned handle iteration: %s", out)
	}
	if !strings.Contains(out, `ctx.scene.graph[digit].set_text("9");`) {
		t.Errorf("loop-bound nodes field handles should expose node methods: %s", out)
	}
}

func TestRewriteNodeChildrenLoopAliases(t *testing.T) {
	body := `let crosshair = self.crosshair;
for reticle_part in crosshair.children() {
    let part_parent = reticle_part.parent();
    reticle_part.set_visibility(true);
    let _ = part_parent;
}`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "crosshair"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, "for reticle_part in ctx.scene.graph[crosshair].children().to_vec().into_iter() {") {
		t.Errorf("node children() iterators should materialize owned handles for authored loops: %s", out)
	}
	if !strings.Contains(out, "let part_parent = ctx.scene.graph[reticle_part].parent();") {
		t.Errorf("loop-bound children handles should expose parent() sugar: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[reticle_part].set_visibility(true);") {
		t.Errorf("loop-bound children handles should expose node methods: %s", out)
	}
}

func TestRewriteNodesFieldBulkMethodCall(t *testing.T) {
	body := `self.ammo_digits.set_text("9");`
	fields := []ast.Field{
		{Modifier: ast.FieldNodes, Name: "ammo_digits"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, "for __fyx_item_0 in self.ammo_digits.iter().cloned() {") {
		t.Errorf("nodes field bulk method calls should lower to collection loops: %s", out)
	}
	if !strings.Contains(out, `ctx.scene.graph[__fyx_item_0].set_text("9");`) {
		t.Errorf("nodes field bulk method calls should target each node handle: %s", out)
	}
}

func TestRewriteNodeChildrenBulkMethodCall(t *testing.T) {
	body := `let crosshair = self.crosshair;
crosshair.children().set_visibility(true);`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "crosshair"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, "for __fyx_item_0 in ctx.scene.graph[crosshair].children().to_vec().into_iter() {") {
		t.Errorf("children() bulk method calls should lower to owned traversal loops: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[__fyx_item_0].set_visibility(true);") {
		t.Errorf("children() bulk method calls should target each child handle: %s", out)
	}
}

func TestRewriteRelativeNodeFindAliases(t *testing.T) {
	body := `let crosshair = self.crosshair;
let reticle_ring = crosshair.find("Reticle/Ring");
let ring_parent = reticle_ring.parent();
reticle_ring.set_visibility(true);`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "crosshair"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, `let reticle_ring = fyx_find_relative_node_path(&ctx.scene.graph, crosshair, "Reticle/Ring");`) {
		t.Errorf("relative find() should lower to the generated scene helper: %s", out)
	}
	if !strings.Contains(out, "let ring_parent = ctx.scene.graph[reticle_ring].parent();") {
		t.Errorf("relative find() aliases should keep handle sugar: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[reticle_ring].set_visibility(true);") {
		t.Errorf("relative find() aliases should keep node methods: %s", out)
	}
}

func TestRewriteTypedRelativeNodeFindAliases(t *testing.T) {
	body := `let crosshair = self.crosshair;
let reticle_ring = crosshair.find::<Sprite>("Reticle/Ring");
reticle_ring.set_visibility(true);`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "crosshair"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, `let reticle_ring = fyx_expect_node_type::<Sprite>(&ctx.scene.graph, fyx_find_relative_node_path(&ctx.scene.graph, crosshair, "Reticle/Ring"), "Reticle/Ring", "Sprite");`) {
		t.Errorf("typed relative find() should lower through fyx_expect_node_type: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[reticle_ring].set_visibility(true);") {
		t.Errorf("typed relative find() aliases should keep node methods: %s", out)
	}
}

func TestRewriteRelativeNodeFindAllLoopAndBulkCall(t *testing.T) {
	body := `let crosshair = self.crosshair;
crosshair.find_all("Marks/*").set_visibility(true);
for mark in crosshair.find_all("Marks/*") {
    let parent = mark.parent();
    mark.set_visibility(false);
    let _ = parent;
}`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "crosshair"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, `for __fyx_item_0 in fyx_find_relative_nodes_path(&ctx.scene.graph, crosshair, "Marks/*").into_iter() {`) {
		t.Errorf("relative find_all() bulk calls should lower to collection loops: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[__fyx_item_0].set_visibility(true);") {
		t.Errorf("relative find_all() bulk calls should target each resolved handle: %s", out)
	}
	if !strings.Contains(out, `for mark in fyx_find_relative_nodes_path(&ctx.scene.graph, crosshair, "Marks/*").into_iter() {`) {
		t.Errorf("relative find_all() loops should lower to iterator traversal: %s", out)
	}
	if !strings.Contains(out, "let parent = ctx.scene.graph[mark].parent();") {
		t.Errorf("relative find_all() loop aliases should keep handle sugar: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[mark].set_visibility(false);") {
		t.Errorf("relative find_all() loop aliases should keep node methods: %s", out)
	}
}

func TestRewriteTypedRelativeNodeFindAllLoopAndBulkCall(t *testing.T) {
	body := `let crosshair = self.crosshair;
crosshair.find_all::<Sprite>("Marks/*").set_visibility(true);
for mark in crosshair.find_all::<Sprite>("Marks/*") {
    mark.set_visibility(false);
}`
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "crosshair"},
	}

	out := RewriteBody(body, "WeaponHUD", fields, ast.HandlerMessage)
	if !strings.Contains(out, `for __fyx_item_0 in fyx_expect_nodes_type::<Sprite>(&ctx.scene.graph, fyx_find_relative_nodes_path(&ctx.scene.graph, crosshair, "Marks/*"), "Marks/*", "Sprite").into_iter() {`) {
		t.Errorf("typed relative find_all() bulk calls should lower through fyx_expect_nodes_type: %s", out)
	}
	if !strings.Contains(out, `for mark in fyx_expect_nodes_type::<Sprite>(&ctx.scene.graph, fyx_find_relative_nodes_path(&ctx.scene.graph, crosshair, "Marks/*"), "Marks/*", "Sprite").into_iter() {`) {
		t.Errorf("typed relative find_all() loops should lower through fyx_expect_nodes_type: %s", out)
	}
}

func TestRewriteGlobalSceneFindAliases(t *testing.T) {
	body := `let reticle_ring = scene.find("UI/Crosshair/Reticle/Ring");
let ring_parent = reticle_ring.parent();
reticle_ring.set_visibility(true);`

	out := RewriteBody(body, "WeaponHUD", nil, ast.HandlerMessage)
	if !strings.Contains(out, `let reticle_ring = fyx_find_node_path(&ctx.scene.graph, "UI/Crosshair/Reticle/Ring");`) {
		t.Errorf("scene.find() should lower to the generated global scene helper: %s", out)
	}
	if !strings.Contains(out, "let ring_parent = ctx.scene.graph[reticle_ring].parent();") {
		t.Errorf("scene.find() aliases should keep handle sugar: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[reticle_ring].set_visibility(true);") {
		t.Errorf("scene.find() aliases should keep node methods: %s", out)
	}
}

func TestRewriteTypedGlobalSceneFindAliases(t *testing.T) {
	body := `let reticle_ring = scene.find::<Sprite>("UI/Crosshair/Reticle/Ring");
reticle_ring.set_visibility(true);`

	out := RewriteBody(body, "WeaponHUD", nil, ast.HandlerMessage)
	if !strings.Contains(out, `let reticle_ring = fyx_expect_node_type::<Sprite>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "UI/Crosshair/Reticle/Ring"), "UI/Crosshair/Reticle/Ring", "Sprite");`) {
		t.Errorf("typed scene.find() should lower through fyx_expect_node_type: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[reticle_ring].set_visibility(true);") {
		t.Errorf("typed scene.find() aliases should keep node methods: %s", out)
	}
}

func TestRewriteGlobalSceneFindAllLoopAndBulkCall(t *testing.T) {
	body := `scene.find_all("UI/Crosshair/Reticle/Marks/*").set_visibility(true);
for mark in scene.find_all("UI/Crosshair/Reticle/Marks/*") {
    let parent = mark.parent();
    mark.set_visibility(false);
    let _ = parent;
}`

	out := RewriteBody(body, "WeaponHUD", nil, ast.HandlerMessage)
	if !strings.Contains(out, `for __fyx_item_0 in fyx_find_nodes_path(&ctx.scene.graph, "UI/Crosshair/Reticle/Marks/*").into_iter() {`) {
		t.Errorf("scene.find_all() bulk calls should lower to collection loops: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[__fyx_item_0].set_visibility(true);") {
		t.Errorf("scene.find_all() bulk calls should target each resolved handle: %s", out)
	}
	if !strings.Contains(out, `for mark in fyx_find_nodes_path(&ctx.scene.graph, "UI/Crosshair/Reticle/Marks/*").into_iter() {`) {
		t.Errorf("scene.find_all() loops should lower to iterator traversal: %s", out)
	}
	if !strings.Contains(out, "let parent = ctx.scene.graph[mark].parent();") {
		t.Errorf("scene.find_all() loop aliases should keep handle sugar: %s", out)
	}
}

func TestRewriteTypedGlobalSceneFindAllLoopAndBulkCall(t *testing.T) {
	body := `scene.find_all::<Sprite>("UI/Crosshair/Reticle/Marks/*").set_visibility(true);
for mark in scene.find_all::<Sprite>("UI/Crosshair/Reticle/Marks/*") {
    mark.set_visibility(false);
}`

	out := RewriteBody(body, "WeaponHUD", nil, ast.HandlerMessage)
	if !strings.Contains(out, `for __fyx_item_0 in fyx_expect_nodes_type::<Sprite>(&ctx.scene.graph, fyx_find_nodes_path(&ctx.scene.graph, "UI/Crosshair/Reticle/Marks/*"), "UI/Crosshair/Reticle/Marks/*", "Sprite").into_iter() {`) {
		t.Errorf("typed scene.find_all() bulk calls should lower through fyx_expect_nodes_type: %s", out)
	}
	if !strings.Contains(out, `for mark in fyx_expect_nodes_type::<Sprite>(&ctx.scene.graph, fyx_find_nodes_path(&ctx.scene.graph, "UI/Crosshair/Reticle/Marks/*"), "UI/Crosshair/Reticle/Marks/*", "Sprite").into_iter() {`) {
		t.Errorf("typed scene.find_all() loops should lower through fyx_expect_nodes_type: %s", out)
	}
}

func TestRewriteScriptContextShorthands(t *testing.T) {
	body := `let hit = scene.physics.raycast(origin, direction, 4.0);
graph.remove_node(doomed);
let prefab = resources.request::<Model>("res://models/projectile.rgs");
messages.send_global(WeaponFiredMsg { position: origin, direction: direction });
dispatcher.subscribe_to::<WeaponFiredMsg>(ctx.handle);
let _ = (hit, prefab);`

	out := RewriteBody(body, "Weapon", nil, ast.HandlerStart)
	if !strings.Contains(out, "let hit = ctx.scene.physics.raycast(origin, direction, 4.0);") {
		t.Errorf("scene shorthand should lower to ctx.scene: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph.remove_node(doomed);") {
		t.Errorf("graph shorthand should lower to ctx.scene.graph: %s", out)
	}
	if !strings.Contains(out, `let prefab = ctx.resource_manager.request::<Model>("res://models/projectile.rgs");`) {
		t.Errorf("resources shorthand should lower to ctx.resource_manager: %s", out)
	}
	if !strings.Contains(out, "ctx.message_sender.send_global(WeaponFiredMsg { position: origin, direction: direction });") {
		t.Errorf("messages shorthand should lower to ctx.message_sender: %s", out)
	}
	if !strings.Contains(out, "ctx.message_dispatcher.subscribe_to::<WeaponFiredMsg>(ctx.handle);") {
		t.Errorf("dispatcher shorthand should lower to ctx.message_dispatcher: %s", out)
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

func TestRewriteTimerSugar(t *testing.T) {
	body := `if fire_cooldown.ready {
    fire_cooldown.reset();
}`
	fields := []ast.Field{
		{Modifier: ast.FieldTimer, Name: "fire_cooldown", TypeExpr: "f32", Default: "self.fire_rate"},
	}

	out := RewriteBody(body, "Weapon", fields, ast.HandlerUpdate)
	if !strings.Contains(out, "if (self.fire_cooldown <= 0.0) {") {
		t.Fatalf("timer ready checks should lower to explicit cooldown tests: %s", out)
	}
	if !strings.Contains(out, "self.fire_cooldown = self.fire_rate;") {
		t.Fatalf("timer reset should lower to the authored reset duration: %s", out)
	}
}

func TestRewriteStateTransitions(t *testing.T) {
	body := `if see_player() {
    go alert;
}`
	states := []ast.State{
		{Name: "idle"},
		{Name: "alert"},
	}

	out := RewriteBodyWithStates(body, "Enemy", nil, states, ast.HandlerUpdate)
	if !strings.Contains(out, "self._fyx_transition = Some(EnemyState::Alert);") {
		t.Fatalf("state transitions should lower to explicit enum transitions: %s", out)
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

func TestRewriteScriptSceneSpawnLifetime(t *testing.T) {
	body := `let projectile = spawn projectile_prefab at origin lifetime 1.0;`
	fields := []ast.Field{
		{Modifier: ast.FieldResource, Name: "projectile_prefab", TypeExpr: "Model"},
		{Modifier: ast.FieldBare, Name: sceneLifetimeFieldName, TypeExpr: sceneLifetimeFieldType},
	}

	out := RewriteBody(body, "Weapon", fields, ast.HandlerUpdate)
	if !strings.Contains(out, "self._fyx_scene_lifetimes.push(FyxSceneLifetime { handle: _inst, remaining: 1.0 });") {
		t.Fatalf("scene spawn lifetime should register auto-cleanup on the owning script: %s", out)
	}
}

func TestRewriteScriptEcsSpawnLifetime(t *testing.T) {
	body := `let shot = ecs.spawn(
    Projectile { damage: 25.0 },
    Velocity { linear: Vector3::default(), angular: Vector3::default() },
) lifetime 1.0;`

	out := RewriteBody(body, "Spawner", nil, ast.HandlerUpdate)
	if !strings.Contains(out, "ctx.ecs.spawn((Projectile { damage: 25.0 }, Velocity { linear: Vector3::default(), angular: Vector3::default() }, FyxEntityLifetime { remaining: 1.0 }))") {
		t.Fatalf("ecs lifetime spawn should append the builtin lifetime component: %s", out)
	}
}
