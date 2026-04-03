package transpiler

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
)

// TranspileComponent generates a Rust struct from an AST Component.
// Component fields are always public and the struct derives Clone.
func TranspileComponent(c ast.Component) string {
	e := NewEmitter()
	e.Line("#[derive(Clone)]")
	e.Linef("pub struct %s {", c.Name)
	e.Indent()
	for _, f := range c.Fields {
		e.Linef("pub %s: %s,", f.Name, f.TypeExpr)
	}
	e.Dedent()
	e.Line("}")
	return e.String()
}

// TranspileSystem generates a Rust function from an AST System.
// Injected parameters (like dt) are bound from ctx, and each query block
// becomes a for loop over world.query_mut.
func TranspileSystem(s ast.System) string {
	e := NewEmitter()
	e.Linef("pub fn system_%s(world: &mut EcsWorld, ctx: &PluginContext) {", s.Name)
	e.Indent()

	// Emit injected parameter bindings.
	for _, p := range s.Params {
		src := injectedParamSource(p)
		if p.TypeExpr != "" {
			e.Linef("let %s: %s = %s;", p.Name, p.TypeExpr, src)
		} else {
			e.Linef("let %s = %s;", p.Name, src)
		}
	}

	// Emit each query block as a for loop.
	for _, q := range s.Queries {
		emitQueryLoop(e, q)
	}

	// Emit any non-query body code.
	body := strings.TrimSpace(s.Body)
	if body != "" {
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			e.Line(strings.TrimRight(line, " \t"))
		}
	}

	e.Dedent()
	e.Line("}")
	return e.String()
}

// TranspileSystemRunner generates the run_ecs_systems function that calls
// each system in declaration order.
func TranspileSystemRunner(systems []ast.System) string {
	e := NewEmitter()
	e.Line("pub fn run_ecs_systems(world: &mut EcsWorld, ctx: &PluginContext) {")
	e.Indent()
	for _, s := range systems {
		e.Linef("system_%s(world, ctx);", s.Name)
	}
	e.Dedent()
	e.Line("}")
	return e.String()
}

// TranspileECS generates all ECS Rust code: component structs, system functions,
// and the system runner, separated by blank lines.
func TranspileECS(components []ast.Component, systems []ast.System) string {
	var parts []string

	for _, c := range components {
		parts = append(parts, TranspileComponent(c))
	}
	for _, s := range systems {
		parts = append(parts, TranspileSystem(s))
	}
	if len(systems) > 0 {
		parts = append(parts, TranspileSystemRunner(systems))
	}

	return strings.Join(parts, "\n")
}

// injectedParamSource returns the Rust expression that provides the value
// for an injected system parameter.
func injectedParamSource(p ast.Param) string {
	switch p.Name {
	case "dt":
		return "ctx.dt"
	default:
		return fmt.Sprintf("ctx.%s", p.Name)
	}
}

// emitQueryLoop emits a for loop that iterates over query results.
// Entity-typed parameters are bound to the entity variable; all others
// go into the query type tuple.
func emitQueryLoop(e *RustEmitter, q ast.Query) {
	var entityVar string
	var queryNames []string
	var queryTypes []string

	for _, p := range q.Params {
		if p.TypeExpr == "Entity" {
			entityVar = p.Name
			continue
		}
		queryNames = append(queryNames, p.Name)
		if p.Mutable {
			queryTypes = append(queryTypes, fmt.Sprintf("&mut %s", p.TypeExpr))
		} else {
			queryTypes = append(queryTypes, fmt.Sprintf("&%s", p.TypeExpr))
		}
	}

	if entityVar == "" {
		entityVar = "_entity"
	}

	typeTuple := strings.Join(queryTypes, ", ")
	nameTuple := strings.Join(queryNames, ", ")

	// Single-element tuples don't need parens in the destructure but do in the type.
	if len(queryNames) == 1 {
		e.Linef("for (%s, %s) in world.query_mut::<(%s,)>() {", entityVar, nameTuple, typeTuple)
	} else {
		e.Linef("for (%s, (%s)) in world.query_mut::<(%s)>() {", entityVar, nameTuple, typeTuple)
	}
	e.Indent()

	body := strings.TrimSpace(q.Body)
	if body != "" {
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			e.Line(strings.TrimRight(line, " \t"))
		}
	}

	e.Dedent()
	e.Line("}")
}
