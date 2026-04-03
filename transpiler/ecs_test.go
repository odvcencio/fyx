package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyrox-lang/ast"
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
	if !strings.Contains(out, "for (e, tag) in") {
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
