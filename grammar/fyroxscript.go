package grammar

import (
	"github.com/odvcencio/gotreesitter/grammargen"
)

// FyroxScriptGrammar returns a grammar that extends Rust with
// FyroxScript-specific productions: script, signal, component, system, etc.
func FyroxScriptGrammar() *grammargen.Grammar {
	g := grammargen.NewGrammar("fyroxscript")

	// Top-level structure
	g.Define("source_file", grammargen.Repeat(grammargen.Sym("_item")))
	g.Define("_item", grammargen.Choice(
		grammargen.Sym("script_declaration"),
		grammargen.Sym("component_declaration"),
		grammargen.Sym("system_declaration"),
		grammargen.Sym("rust_item"),
	))

	// Rust passthrough: any top-level Rust construct that isn't script/component/system.
	// Two forms:
	//   - Statement: rust_keyword ... ;  (e.g. use, const, static, type alias)
	//   - Block:     rust_keyword ... { balanced } (e.g. fn, struct, enum, impl, trait)
	g.Define("rust_item", grammargen.Seq(
		grammargen.Sym("_rust_keyword"),
		grammargen.Repeat(grammargen.Token(grammargen.Pat(`[^{};]+`))),
		grammargen.Choice(
			grammargen.Str(";"),
			grammargen.Sym("_nested_brace_block"),
		),
	))

	// Rust top-level keywords (excluding script, component, system)
	g.Define("_rust_keyword", grammargen.Choice(
		grammargen.Str("use"),
		grammargen.Str("fn"),
		grammargen.Str("pub"),
		grammargen.Str("struct"),
		grammargen.Str("enum"),
		grammargen.Str("impl"),
		grammargen.Str("trait"),
		grammargen.Str("type"),
		grammargen.Str("const"),
		grammargen.Str("static"),
		grammargen.Str("mod"),
		grammargen.Str("extern"),
		grammargen.Str("unsafe"),
		grammargen.Str("async"),
		grammargen.Str("#"),
	))

	// Script declaration with optional body members (fields + lifecycle handlers)
	g.Define("script_declaration", grammargen.Seq(
		grammargen.Str("script"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("_script_member")),
		grammargen.Str("}"),
	))

	// Component declaration: component Velocity { linear: Vector3\n angular: Vector3\n }
	// Contains bare fields only (no modifiers), newline-separated.
	g.Define("component_declaration", grammargen.Seq(
		grammargen.Str("component"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("component_field")),
		grammargen.Str("}"),
	))

	// Component field: name: Type (bare field, no modifier, no default)
	g.Define("component_field", grammargen.Seq(
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
	))

	// System declaration: system name(injected_params) { body }
	// Optional injected params. Body can contain query blocks and arbitrary code.
	g.Define("system_declaration", grammargen.Seq(
		grammargen.Str("system"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("("),
			grammargen.Optional(grammargen.Sym("system_parameters")),
			grammargen.Str(")"),
		)),
		grammargen.Field("body", grammargen.Sym("system_body")),
	))

	// System parameter list: param1: Type, param2: Type
	g.Define("system_parameters", grammargen.Seq(
		grammargen.Sym("handler_parameter"),
		grammargen.Repeat(grammargen.Seq(
			grammargen.Str(","),
			grammargen.Sym("handler_parameter"),
		)),
	))

	// System body: balanced braces containing query blocks and arbitrary code
	g.Define("system_body", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("_system_body_content")),
		grammargen.Str("}"),
	))

	// System body content: query blocks, nested brace blocks, or raw text.
	// Raw text uses a single-char pattern [^{}] (not [^{}]+) so that the
	// lexer does not greedily consume the "query" keyword before the parser
	// can try the query_block alternative.
	g.Define("_system_body_content", grammargen.Choice(
		grammargen.Sym("query_block"),
		grammargen.Sym("_nested_brace_block"),
		grammargen.Token(grammargen.Pat(`[^{}]`)),
	))

	// Query block: query(params) { body }
	g.Define("query_block", grammargen.Seq(
		grammargen.Str("query"),
		grammargen.Str("("),
		grammargen.Optional(grammargen.Sym("query_parameters")),
		grammargen.Str(")"),
		grammargen.Field("body", grammargen.Sym("handler_body")),
	))

	// Query parameter list: param1: &mut Type, param2: &Type, param3: Type
	g.Define("query_parameters", grammargen.Seq(
		grammargen.Sym("query_parameter"),
		grammargen.Repeat(grammargen.Seq(
			grammargen.Str(","),
			grammargen.Sym("query_parameter"),
		)),
	))

	// Query parameter: name: &mut Type | name: &Type | name: Type
	g.Define("query_parameter", grammargen.Seq(
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("query_type")),
	))

	// Query type: &mut Type | &Type | Type
	g.Define("query_type", grammargen.Choice(
		grammargen.Seq(grammargen.Str("&"), grammargen.Str("mut"), grammargen.Sym("type_expression")),
		grammargen.Seq(grammargen.Str("&"), grammargen.Sym("type_expression")),
		grammargen.Sym("type_expression"),
	))

	// A script member is a field declaration, lifecycle handler, signal, connect block, or watch block
	g.Define("_script_member", grammargen.Choice(
		grammargen.Sym("_field_declaration"),
		grammargen.Sym("lifecycle_handler"),
		grammargen.Sym("signal_declaration"),
		grammargen.Sym("connect_block"),
		grammargen.Sym("watch_block"),
	))

	// Field declarations — each modifier variant is its own node type
	g.Define("_field_declaration", grammargen.Choice(
		grammargen.Sym("inspect_field"),
		grammargen.Sym("node_field"),
		grammargen.Sym("nodes_field"),
		grammargen.Sym("resource_field"),
		grammargen.Sym("reactive_field"),
		grammargen.Sym("derived_field"),
		grammargen.Sym("bare_field"),
	))

	// inspect speed: f32 = 10.0
	g.Define("inspect_field", grammargen.Seq(
		grammargen.Str("inspect"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("="),
			grammargen.Field("default", grammargen.Sym("default_value")),
		)),
	))

	// node camera: Camera3D = "Camera3D"
	g.Define("node_field", grammargen.Seq(
		grammargen.Str("node"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("="),
			grammargen.Field("default", grammargen.Sym("default_value")),
		)),
	))

	// nodes gears: Mesh = "Gears/*"
	g.Define("nodes_field", grammargen.Seq(
		grammargen.Str("nodes"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("="),
			grammargen.Field("default", grammargen.Sym("default_value")),
		)),
	))

	// resource footstep: SoundBuffer = "res://audio/footstep.wav"
	g.Define("resource_field", grammargen.Seq(
		grammargen.Str("resource"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("="),
			grammargen.Field("default", grammargen.Sym("default_value")),
		)),
	))

	// move_dir: Vector3  (bare field — no modifier keyword)
	g.Define("bare_field", grammargen.Seq(
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("="),
			grammargen.Field("default", grammargen.Sym("default_value")),
		)),
	))

	// reactive health: f32 = 100.0
	g.Define("reactive_field", grammargen.Seq(
		grammargen.Str("reactive"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str("="),
			grammargen.Field("default", grammargen.Sym("default_value")),
		)),
	))

	// derived health_pct: f32 = self.health / 100.0
	g.Define("derived_field", grammargen.Seq(
		grammargen.Str("derived"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("type", grammargen.Sym("type_expression")),
		grammargen.Str("="),
		grammargen.Field("expression", grammargen.Sym("default_value")),
	))

	// watch self.is_critical { do_something(); }
	g.Define("watch_block", grammargen.Seq(
		grammargen.Str("watch"),
		grammargen.Field("target", grammargen.Sym("watch_target")),
		grammargen.Field("body", grammargen.Sym("handler_body")),
	))

	// Watch target: self.field_name
	g.Define("watch_target", grammargen.Seq(
		grammargen.Str("self"),
		grammargen.Str("."),
		grammargen.Field("field", grammargen.Sym("identifier")),
	))

	// Lifecycle handler: on init(ctx) { ... }
	g.Define("lifecycle_handler", grammargen.Seq(
		grammargen.Str("on"),
		grammargen.Field("kind", grammargen.Sym("handler_kind")),
		grammargen.Str("("),
		grammargen.Optional(grammargen.Sym("handler_parameters")),
		grammargen.Str(")"),
		grammargen.Field("body", grammargen.Sym("handler_body")),
	))

	// Handler kinds: init, start, update, deinit, event, message
	g.Define("handler_kind", grammargen.Choice(
		grammargen.Str("init"),
		grammargen.Str("start"),
		grammargen.Str("update"),
		grammargen.Str("deinit"),
		grammargen.Str("event"),
		grammargen.Str("message"),
	))

	// Handler parameter list: param1, param2: Type, ...
	g.Define("handler_parameters", grammargen.Seq(
		grammargen.Sym("handler_parameter"),
		grammargen.Repeat(grammargen.Seq(
			grammargen.Str(","),
			grammargen.Sym("handler_parameter"),
		)),
	))

	// A single handler parameter: name or name: Type
	// Uses param_type_expression (not type_expression) to avoid LALR state
	// merging between field-context and param-context type parsing.
	g.Define("handler_parameter", grammargen.Seq(
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str(":"),
			grammargen.Field("type", grammargen.Sym("param_type_expression")),
		)),
	))

	// Handler body: balanced braces containing arbitrary Rust code
	g.Define("handler_body", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("_body_content")),
		grammargen.Str("}"),
	))

	// Body content: anything that isn't a brace, or a nested brace block
	g.Define("_body_content", grammargen.Choice(
		grammargen.Sym("_nested_brace_block"),
		grammargen.Token(grammargen.Pat(`[^{}]+`)),
	))

	// Nested brace block for balanced brace matching
	g.Define("_nested_brace_block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("_body_content")),
		grammargen.Str("}"),
	))

	// Signal declaration: signal died(position: Vector3)
	// Reuses handler_parameters for the param list to avoid LR conflicts
	// with type_expression generics (<>) in parallel param rules.
	g.Define("signal_declaration", grammargen.Seq(
		grammargen.Str("signal"),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Str("("),
		grammargen.Optional(grammargen.Sym("handler_parameters")),
		grammargen.Str(")"),
	))

	// Connect block: connect Enemy::died(pos) { ... }
	// Reuses handler_parameters for the param list.
	g.Define("connect_block", grammargen.Seq(
		grammargen.Str("connect"),
		grammargen.Field("signal", grammargen.Sym("signal_path")),
		grammargen.Str("("),
		grammargen.Optional(grammargen.Sym("handler_parameters")),
		grammargen.Str(")"),
		grammargen.Field("body", grammargen.Sym("handler_body")),
	))

	// Signal path: ScriptName::signal_name
	g.Define("signal_path", grammargen.Seq(
		grammargen.Field("script", grammargen.Sym("identifier")),
		grammargen.Str("::"),
		grammargen.Field("name", grammargen.Sym("identifier")),
	))

	// Type expressions: f32, Vector3, Handle<Node>, Vec<Handle<Node>>, etc.
	g.Define("type_expression", grammargen.Seq(
		grammargen.Sym("identifier"),
		grammargen.Optional(grammargen.Sym("type_arguments")),
	))

	// Generic type arguments: <Node>, <Handle<Node>>
	g.Define("type_arguments", grammargen.Seq(
		grammargen.Str("<"),
		grammargen.Sym("type_expression"),
		grammargen.Repeat(grammargen.Seq(
			grammargen.Str(","),
			grammargen.Sym("type_expression"),
		)),
		grammargen.Str(">"),
	))

	// Parameter type expressions — structurally identical to type_expression
	// but a separate symbol to prevent LALR state merging between field-context
	// and parenthesized-param-context, which causes misparses of ">" before ")".
	g.Define("param_type_expression", grammargen.Seq(
		grammargen.Sym("identifier"),
		grammargen.Optional(grammargen.Sym("param_type_arguments")),
	))

	g.Define("param_type_arguments", grammargen.Seq(
		grammargen.Str("<"),
		grammargen.Sym("param_type_expression"),
		grammargen.Repeat(grammargen.Seq(
			grammargen.Str(","),
			grammargen.Sym("param_type_expression"),
		)),
		grammargen.Str(">"),
	))

	// Default value: capture everything after = until newline.
	// Uses a pattern that matches any non-empty sequence of non-newline chars.
	g.Define("default_value", grammargen.Token(grammargen.Pat(`[^\n]+`)))

	// Identifiers
	g.Define("identifier", grammargen.Pat(`[a-zA-Z_][a-zA-Z0-9_]*`))

	g.SetExtras(grammargen.Pat(`\s`))

	return g
}
