package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
)

var despawnCallRe = regexp.MustCompile(`\bdespawn\s*\(`)

// TranspileComponent generates a Rust struct from an AST Component.
// Component fields are always public and the struct derives Clone.
func TranspileComponent(c ast.Component) string {
	e := NewEmitter()
	EmitComponent(e, c)
	return e.String()
}

// EmitComponent writes a component struct into an emitter.
func EmitComponent(e *RustEmitter, c ast.Component) {
	e.LineWithSource("#[derive(Clone)]", c.Line)
	e.LineWithSource(fmt.Sprintf("pub struct %s {", c.Name), c.Line)
	e.Indent()
	for _, f := range c.Fields {
		e.LineWithSource(fmt.Sprintf("pub %s: %s,", f.Name, f.TypeExpr), f.Line)
	}
	e.Dedent()
	e.LineWithSource("}", c.Line)
}

// TranspileSystem generates a Rust function from an AST System.
// Injected parameters (like dt) are bound from ctx, and each query block
// becomes a for loop over world.query_mut.
func TranspileSystem(s ast.System) string {
	e := NewEmitter()
	EmitSystem(e, s)
	return e.String()
}

// EmitSystem writes a system function into an emitter.
func EmitSystem(e *RustEmitter, s ast.System) {
	e.LineWithSource(fmt.Sprintf("pub fn system_%s(world: &mut EcsWorld, ctx: &PluginContext) {", s.Name), s.Line)
	e.Indent()

	for _, p := range s.Params {
		src := injectedParamSource(p)
		if p.TypeExpr != "" {
			e.LineWithSource(fmt.Sprintf("let %s: %s = %s;", p.Name, p.TypeExpr, src), s.Line)
		} else {
			e.LineWithSource(fmt.Sprintf("let %s = %s;", p.Name, src), s.Line)
		}
	}

	for _, q := range s.Queries {
		emitQueryLoop(e, q)
	}

	body := strings.TrimSpace(rewriteSystemBody(s.Body))
	if body != "" {
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			e.LineWithSource(strings.TrimRight(line, " \t"), s.BodyLine+i)
		}
	}

	e.Dedent()
	e.LineWithSource("}", s.Line)
}

// TranspileSystemRunner generates the run_ecs_systems function that calls
// each system in declaration order.
func TranspileSystemRunner(systems []ast.System) string {
	e := NewEmitter()
	EmitSystemRunner(e, systems)
	return e.String()
}

// EmitSystemRunner writes the ECS runner function into an emitter.
func EmitSystemRunner(e *RustEmitter, systems []ast.System) {
	line := 1
	if len(systems) > 0 && systems[0].Line != 0 {
		line = systems[0].Line
	}
	e.LineWithSource("pub fn run_ecs_systems(world: &mut EcsWorld, ctx: &PluginContext) {", line)
	e.Indent()
	for _, s := range systems {
		e.LineWithSource(fmt.Sprintf("system_%s(world, ctx);", s.Name), s.Line)
	}
	e.Dedent()
	e.LineWithSource("}", line)
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
		e.LineWithSource(fmt.Sprintf("for (%s, (%s,)) in world.query_mut::<(%s,)>() {", entityVar, nameTuple, typeTuple), q.Line)
	} else {
		e.LineWithSource(fmt.Sprintf("for (%s, (%s)) in world.query_mut::<(%s)>() {", entityVar, nameTuple, typeTuple), q.Line)
	}
	e.Indent()

		body := strings.TrimSpace(rewriteSystemBody(q.Body))
		if body != "" {
			lines := strings.Split(body, "\n")
			for i, line := range lines {
				e.LineWithSource(strings.TrimRight(line, " \t"), q.BodyLine+i)
			}
		}

	e.Dedent()
	e.LineWithSource("}", q.Line)
}

func rewriteSystemBody(body string) string {
	return despawnCallRe.ReplaceAllString(body, "world.despawn(")
}
