package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
)

func TestTranspileComponent(t *testing.T) {
	c := ast.Component{
		Name: "Velocity",
		Fields: []ast.Field{
			{Modifier: ast.FieldBare, Name: "linear", TypeExpr: "Vector3"},
			{Modifier: ast.FieldBare, Name: "angular", TypeExpr: "Vector3"},
		},
	}
	out := TranspileComponent(c)
	if !strings.Contains(out, "#[derive(Clone)]") {
		t.Errorf("missing derive Clone: %s", out)
	}
	if !strings.Contains(out, "pub struct Velocity") {
		t.Errorf("missing pub struct: %s", out)
	}
	if !strings.Contains(out, "pub linear: Vector3,") {
		t.Errorf("missing linear field: %s", out)
	}
	if !strings.Contains(out, "pub angular: Vector3,") {
		t.Errorf("missing angular field: %s", out)
	}
}

func TestTranspileSystem(t *testing.T) {
	s := ast.System{
		Name:   "move_things",
		Params: []ast.Param{{Name: "dt", TypeExpr: "f32"}},
		Queries: []ast.Query{
			{
				Params: []ast.QueryParam{
					{Name: "pos", Mutable: true, TypeExpr: "Transform"},
					{Name: "vel", Mutable: false, TypeExpr: "Velocity"},
				},
				Body: "pos.translate(vel.linear * dt);",
			},
		},
	}
	out := TranspileSystem(s)
	if !strings.Contains(out, "pub fn system_move_things(world: &mut EcsWorld, ctx: &PluginContext)") {
		t.Errorf("missing system function signature: %s", out)
	}
	if !strings.Contains(out, "let dt: f32 = ctx.dt;") {
		t.Errorf("missing dt binding: %s", out)
	}
	if !strings.Contains(out, "query_mut::<(&mut Transform, &Velocity)>()") {
		t.Errorf("missing query_mut with correct types: %s", out)
	}
	if !strings.Contains(out, "pos.translate(vel.linear * dt);") {
		t.Errorf("missing query body: %s", out)
	}
}

func TestTranspileSystemEntityParam(t *testing.T) {
	s := ast.System{
		Name: "tag_entities",
		Queries: []ast.Query{
			{
				Params: []ast.QueryParam{
					{Name: "e", Mutable: false, TypeExpr: "Entity"},
					{Name: "tag", Mutable: false, TypeExpr: "Tag"},
				},
				Body: "process(e, tag);",
			},
		},
	}
	out := TranspileSystem(s)
	if !strings.Contains(out, "for (e, (tag,)) in") {
		t.Errorf("entity param should be bound as entity variable: %s", out)
	}
	// Entity should not appear in the query type tuple
	if strings.Contains(out, "&Entity") {
		t.Errorf("Entity should not appear in query type tuple: %s", out)
	}
}

func TestTranspileSystemRunner(t *testing.T) {
	systems := []ast.System{
		{Name: "move_things"},
		{Name: "expire_things"},
	}
	out := TranspileSystemRunner(systems)
	if !strings.Contains(out, "pub fn run_ecs_systems") {
		t.Errorf("missing runner function: %s", out)
	}
	if !strings.Contains(out, "system_move_things(world, ctx);") {
		t.Errorf("missing first system call: %s", out)
	}
	if !strings.Contains(out, "system_expire_things(world, ctx);") {
		t.Errorf("missing second system call: %s", out)
	}
	// Ensure ordering: move before expire
	moveIdx := strings.Index(out, "system_move_things")
	expireIdx := strings.Index(out, "system_expire_things")
	if moveIdx > expireIdx {
		t.Errorf("systems should be called in declaration order: %s", out)
	}
}

func TestTranspileECS(t *testing.T) {
	components := []ast.Component{
		{Name: "Velocity", Fields: []ast.Field{
			{Modifier: ast.FieldBare, Name: "linear", TypeExpr: "Vector3"},
			{Modifier: ast.FieldBare, Name: "angular", TypeExpr: "Vector3"},
		}},
	}
	systems := []ast.System{
		{
			Name:   "move_things",
			Params: []ast.Param{{Name: "dt", TypeExpr: "f32"}},
			Queries: []ast.Query{
				{
					Params: []ast.QueryParam{
						{Name: "pos", Mutable: true, TypeExpr: "Transform"},
						{Name: "vel", Mutable: false, TypeExpr: "Velocity"},
					},
					Body: "pos.translate(vel.linear * dt);",
				},
			},
		},
	}
	out := TranspileECS(components, systems)
	if !strings.Contains(out, "struct Velocity") {
		t.Errorf("missing component struct: %s", out)
	}
	if !strings.Contains(out, "fn system_move_things") {
		t.Errorf("missing system function: %s", out)
	}
	if !strings.Contains(out, "query_mut") {
		t.Errorf("missing query_mut: %s", out)
	}
	if !strings.Contains(out, "run_ecs_systems") {
		t.Errorf("missing system runner: %s", out)
	}
}

func TestTranspileSystemSingleQueryParam(t *testing.T) {
	s := ast.System{
		Name: "tick_timers",
		Queries: []ast.Query{
			{
				Params: []ast.QueryParam{
					{Name: "timer", Mutable: true, TypeExpr: "Timer"},
				},
				Body: "timer.tick();",
			},
		},
	}
	out := TranspileSystem(s)
	// Single-element query should still work
	if !strings.Contains(out, "query_mut::<(&mut Timer,)>()") {
		t.Errorf("single-element query should have trailing comma in type: %s", out)
	}
}

func TestTranspileECSNoSystems(t *testing.T) {
	components := []ast.Component{
		{Name: "Health", Fields: []ast.Field{
			{Modifier: ast.FieldBare, Name: "current", TypeExpr: "f32"},
		}},
	}
	out := TranspileECS(components, nil)
	if !strings.Contains(out, "struct Health") {
		t.Errorf("missing component struct: %s", out)
	}
	// No systems means no runner
	if strings.Contains(out, "run_ecs_systems") {
		t.Errorf("should not have runner when no systems: %s", out)
	}
}

func TestTranspileSystemMultipleQueries(t *testing.T) {
	s := ast.System{
		Name: "dual_query",
		Queries: []ast.Query{
			{
				Params: []ast.QueryParam{
					{Name: "a", Mutable: true, TypeExpr: "Alpha"},
				},
				Body: "a.process();",
			},
			{
				Params: []ast.QueryParam{
					{Name: "b", Mutable: false, TypeExpr: "Beta"},
				},
				Body: "b.read();",
			},
		},
	}
	out := TranspileSystem(s)
	if !strings.Contains(out, "Alpha") {
		t.Errorf("missing first query type: %s", out)
	}
	if !strings.Contains(out, "Beta") {
		t.Errorf("missing second query type: %s", out)
	}
}

func TestTranspileSystemRewritesEcsSpawn(t *testing.T) {
	s := ast.System{
		Name: "spawn_projectiles",
		Body: `let projectile = ecs.spawn(
    Projectile { damage: 5.0, lifetime: dt },
    Velocity { linear: Vector3::default(), angular: Vector3::default() },
);`,
		Params: []ast.Param{{Name: "dt", TypeExpr: "f32"}},
	}
	out := TranspileSystem(s)
	if !strings.Contains(out, "world.spawn((Projectile { damage: 5.0, lifetime: dt }, Velocity { linear: Vector3::default(), angular: Vector3::default() }))") {
		t.Errorf("ecs.spawn should be rewritten to world.spawn tuple: %s", out)
	}
}

func TestTranspileFileSystemRewritesQualifiedEmit(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Enemy",
				Signals: []ast.Signal{
					{Name: "damaged", Params: []ast.Param{{Name: "amount", TypeExpr: "f32"}}},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "damage_enemies",
				Body: "emit Enemy::damaged(amount: 5.0) to target;",
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "ctx.message_sender.send_to_target(target, EnemyDamagedMsg { amount: 5.0 });") {
		t.Fatalf("qualified emit should be rewritten inside systems: %s", out)
	}
	if strings.Contains(out, "emit Enemy::damaged") {
		t.Fatalf("raw emit should not remain in system output: %s", out)
	}
}

func TestTranspileFileSystemRewritesSceneShorthandAndTargetedEmit(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Damageable",
				Signals: []ast.Signal{
					{
						Name: "damaged",
						Params: []ast.Param{
							{Name: "amount", TypeExpr: "f32"},
							{Name: "source", TypeExpr: "Handle<Node>"},
						},
					},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "projectile_hits",
				Queries: []ast.Query{
					{
						Params: []ast.QueryParam{
							{Name: "entity", TypeExpr: "Entity"},
							{Name: "pos", TypeExpr: "Transform"},
							{Name: "vel", TypeExpr: "Velocity"},
							{Name: "proj", TypeExpr: "Projectile"},
						},
						Body: `for hit in scene.physics.raycast(pos.position(), vel.linear.normalized(), 0.5) {
    emit Damageable::damaged(amount: proj.damage, source: proj.owner) to hit.node;
    despawn(entity);
}`,
					},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "for hit in ctx.scene.physics.raycast(pos.position(), vel.linear.normalized(), 0.5) {") {
		t.Fatalf("scene shorthand should rewrite to ctx.scene inside systems: %s", out)
	}
	if !strings.Contains(out, "ctx.message_sender.send_to_target(hit.node, DamageableDamagedMsg { amount: proj.damage, source: proj.owner });") {
		t.Fatalf("targeted emit should be rewritten inside system query bodies: %s", out)
	}
	if !strings.Contains(out, "world.despawn(entity);") {
		t.Fatalf("despawn shorthand should remain active inside the same system body: %s", out)
	}
}

func TestTranspileFileSystemRewritesComponentNodeHandleSugar(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Damageable",
				Signals: []ast.Signal{
					{Name: "damaged", Params: []ast.Param{{Name: "amount", TypeExpr: "f32"}}},
				},
			},
		},
		Components: []ast.Component{
			{
				Name: "ShotOwner",
				Fields: []ast.Field{
					{Modifier: ast.FieldBare, Name: "node", TypeExpr: "Handle<Node>"},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "inspect_owners",
				Queries: []ast.Query{
					{
						Params: []ast.QueryParam{
							{Name: "owner", TypeExpr: "ShotOwner"},
						},
						Body: `let pos = owner.node.position();
if let Some(target) = owner.node.script::<Damageable>() {
    let _ = (pos, target);
}
owner.node.set_visibility(false);
owner.node.rotate_y(0.25);`,
					},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "let pos = ctx.scene.graph[owner.node].global_position();") {
		t.Fatalf("component Handle<Node> fields should expose position() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "if let Some(target) = ctx.scene.graph[owner.node].script::<Damageable>() {") {
		t.Fatalf("component Handle<Node> fields should expose script::<T>() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[owner.node].set_visibility(false);") {
		t.Fatalf("component Handle<Node> fields should expose node methods inside systems: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[owner.node].set_rotation_y(0.25);") {
		t.Fatalf("component Handle<Node> fields should reuse graph rotate sugar inside systems: %s", out)
	}
}

func TestTranspileFileSystemRewritesComponentNodeHandleAliases(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Damageable",
			},
			{
				Name: "Weapon",
			},
		},
		Components: []ast.Component{
			{
				Name: "ShotOwner",
				Fields: []ast.Field{
					{Modifier: ast.FieldBare, Name: "node", TypeExpr: "Handle<Node>"},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "inspect_owners",
				Queries: []ast.Query{
					{
						Params: []ast.QueryParam{
							{Name: "owner", TypeExpr: "ShotOwner"},
						},
						Body: `let owner_node = owner.node;
let owner_parent = owner_node.parent();
let pos = owner_node.position();
if let Some(weapon) = owner_parent.script::<Weapon>() {
    let _ = (pos, weapon);
}
owner_node.set_visibility(false);`,
					},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "let owner_node = owner.node;") {
		t.Fatalf("component handle aliases should preserve the authored handle binding: %s", out)
	}
	if !strings.Contains(out, "let owner_parent = ctx.scene.graph[owner_node].parent();") {
		t.Fatalf("component handle aliases should keep parent() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "let pos = ctx.scene.graph[owner_node].global_position();") {
		t.Fatalf("component handle aliases should keep position() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "if let Some(weapon) = ctx.scene.graph[owner_parent].script::<Weapon>() {") {
		t.Fatalf("component handle aliases should keep script::<T>() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[owner_node].set_visibility(false);") {
		t.Fatalf("component handle aliases should keep node method sugar inside systems: %s", out)
	}
}

func TestTranspileFileSystemRewritesComponentNodeHandleCollections(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "Weapon",
			},
			{
				Name: "Damageable",
				Signals: []ast.Signal{
					{
						Name: "damaged",
						Params: []ast.Param{
							{Name: "amount", TypeExpr: "f32"},
							{Name: "source", TypeExpr: "Handle<Node>"},
						},
					},
				},
			},
		},
		Components: []ast.Component{
			{
				Name: "PatrolPath",
				Fields: []ast.Field{
					{Modifier: ast.FieldBare, Name: "points", TypeExpr: "Vec<Handle<Node>>"},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "inspect_path",
				Queries: []ast.Query{
					{
						Params: []ast.QueryParam{
							{Name: "path", TypeExpr: "PatrolPath"},
						},
						Body: `let first_point = path.points[0];
let first_pos = first_point.position();
let next_pos = path.points[1].position();
if let Some(weapon) = first_point.parent().script::<Weapon>() {
    let _ = (first_pos, next_pos, weapon);
}
emit Damageable::damaged(amount: 1.0, source: first_point) to path.points;
path.points.set_visibility(false);
for point in path.points {
    let point_parent = point.parent();
    point.set_visibility(false);
    let _ = point_parent;
}`,
					},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "let first_pos = ctx.scene.graph[first_point].global_position();") {
		t.Fatalf("component Vec<Handle<Node>> aliases should keep position() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "let next_pos = ctx.scene.graph[path.points[1]].global_position();") {
		t.Fatalf("component Vec<Handle<Node>> indexed entries should keep position() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "if let Some(weapon) = ctx.scene.graph[ctx.scene.graph[first_point].parent()].script::<Weapon>() {") {
		t.Fatalf("component Vec<Handle<Node>> aliases should keep parent/script sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "for __fyx_target_0 in path.points.iter().cloned() {") {
		t.Fatalf("component Vec<Handle<Node>> targeted emits should lower to collection broadcasts: %s", out)
	}
	if !strings.Contains(out, "ctx.message_sender.send_to_target(__fyx_target_0, DamageableDamagedMsg { amount: 1.0, source: first_point });") {
		t.Fatalf("component Vec<Handle<Node>> targeted emits should target each collection handle: %s", out)
	}
	if !strings.Contains(out, "for __fyx_item_0 in path.points.iter().cloned() {") {
		t.Fatalf("component Vec<Handle<Node>> bulk method calls should lower to collection loops: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[__fyx_item_0].set_visibility(false);") {
		t.Fatalf("component Vec<Handle<Node>> bulk method calls should target each node handle: %s", out)
	}
	if !strings.Contains(out, "for point in path.points.iter().cloned() {") {
		t.Fatalf("bare component Vec<Handle<Node>> loops should lower to cloned handle iteration: %s", out)
	}
	if !strings.Contains(out, "let point_parent = ctx.scene.graph[point].parent();") {
		t.Fatalf("loop-bound component Vec<Handle<Node>> entries should keep parent() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[point].set_visibility(false);") {
		t.Fatalf("loop-bound component Vec<Handle<Node>> entries should keep node method sugar inside systems: %s", out)
	}
}

func TestTranspileFileSystemRewritesHandleChildrenLoops(t *testing.T) {
	file := ast.File{
		Components: []ast.Component{
			{
				Name: "ShotOwner",
				Fields: []ast.Field{
					{Modifier: ast.FieldBare, Name: "node", TypeExpr: "Handle<Node>"},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "inspect_owner_children",
				Queries: []ast.Query{
					{
						Params: []ast.QueryParam{
							{Name: "owner", TypeExpr: "ShotOwner"},
						},
						Body: `owner.node.children().set_visibility(false);
for child in owner.node.children() {
    let child_parent = child.parent();
    child.set_visibility(false);
    let _ = child_parent;
}`,
					},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "for __fyx_item_0 in ctx.scene.graph[owner.node].children().to_vec().into_iter() {") {
		t.Fatalf("component handle children() bulk method calls should lower to owned traversal loops: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[__fyx_item_0].set_visibility(false);") {
		t.Fatalf("component handle children() bulk method calls should target each child handle: %s", out)
	}
	if !strings.Contains(out, "for child in ctx.scene.graph[owner.node].children().to_vec().into_iter() {") {
		t.Fatalf("component handle children() iterators should materialize owned handles inside systems: %s", out)
	}
	if !strings.Contains(out, "let child_parent = ctx.scene.graph[child].parent();") {
		t.Fatalf("loop-bound children handles should keep parent() sugar inside systems: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[child].set_visibility(false);") {
		t.Fatalf("loop-bound children handles should keep node method sugar inside systems: %s", out)
	}
}

func TestTranspileFileSystemRewritesRelativeSceneTraversal(t *testing.T) {
	file := ast.File{
		Components: []ast.Component{
			{
				Name: "ShotOwner",
				Fields: []ast.Field{
					{Modifier: ast.FieldBare, Name: "node", TypeExpr: "Handle<Node>"},
				},
			},
		},
		Systems: []ast.System{
			{
				Name: "inspect_owner_descendants",
				Queries: []ast.Query{
					{
						Params: []ast.QueryParam{
							{Name: "owner", TypeExpr: "ShotOwner"},
						},
						Body: `let trail_root = owner.node.find("TrailRoot");
let trail_root_parent = trail_root.parent();
owner.node.find_all("TrailRoot/*").set_visibility(false);
for segment in owner.node.find_all("TrailRoot/*") {
    let segment_parent = segment.parent();
    segment.set_visibility(true);
    let _ = (trail_root_parent, segment_parent);
}`,
					},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, `let trail_root = fyx_find_relative_node_path(&ctx.scene.graph, owner.node, "TrailRoot");`) {
		t.Fatalf("component handle relative find() should lower to the generated scene helper: %s", out)
	}
	if !strings.Contains(out, "let trail_root_parent = ctx.scene.graph[trail_root].parent();") {
		t.Fatalf("component handle relative find() aliases should keep handle sugar: %s", out)
	}
	if !strings.Contains(out, `for __fyx_item_0 in fyx_find_relative_nodes_path(&ctx.scene.graph, owner.node, "TrailRoot/*").into_iter() {`) {
		t.Fatalf("component handle relative find_all() bulk calls should lower to collection loops: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[__fyx_item_0].set_visibility(false);") {
		t.Fatalf("component handle relative find_all() bulk calls should target each resolved handle: %s", out)
	}
	if !strings.Contains(out, `for segment in fyx_find_relative_nodes_path(&ctx.scene.graph, owner.node, "TrailRoot/*").into_iter() {`) {
		t.Fatalf("component handle relative find_all() loops should lower to iterator traversal: %s", out)
	}
	if !strings.Contains(out, "let segment_parent = ctx.scene.graph[segment].parent();") {
		t.Fatalf("component handle relative find_all() loops should keep handle sugar: %s", out)
	}
	if !strings.Contains(out, "ctx.scene.graph[segment].set_visibility(true);") {
		t.Fatalf("component handle relative find_all() loops should keep node methods: %s", out)
	}
}
