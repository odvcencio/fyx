package transpiler

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
)

// TranspileScript takes an AST Script and returns the corresponding Rust source code.
// It generates a struct with derive macros, a Default impl (if any fields have defaults),
// and a ScriptTrait impl with lifecycle handlers.
func TranspileScript(s ast.Script) string {
	e := NewEmitter()

	emitStruct(e, s)
	e.Blank()

	if hasDefaults(s) {
		emitDefaultImpl(e, s)
		e.Blank()
	}

	emitScriptTraitImpl(e, s)

	return e.String()
}

// scriptUUID generates a deterministic UUID from the script name using SHA-256.
func scriptUUID(name string) string {
	h := sha256.Sum256([]byte(name))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4],
		h[4:6],
		h[6:8],
		h[8:10],
		h[10:16],
	)
}

// hasDefaults returns true if any non-node/resource field in the script has a non-empty Default value.
// Node, Nodes, and Resource fields use their Default as a runtime path/name, not a struct default.
func hasDefaults(s ast.Script) bool {
	for _, f := range s.Fields {
		if f.Default != "" && !isRuntimeResolved(f) {
			return true
		}
	}
	return false
}

// isRuntimeResolved returns true for field modifiers whose defaults are resolved at runtime
// (in on_start) rather than being struct field defaults.
func isRuntimeResolved(f ast.Field) bool {
	switch f.Modifier {
	case ast.FieldNode, ast.FieldNodes, ast.FieldResource:
		return true
	}
	return false
}

// emitStruct emits the struct definition with derive macros and field annotations.
func emitStruct(e *RustEmitter, s ast.Script) {
	e.Line("#[derive(Visit, Reflect, Default, Debug, Clone, TypeUuidProvider, ComponentProvider)]")
	e.Linef("#[type_uuid(id = \"%s\")]", scriptUUID(s.Name))
	e.Line("#[visit(optional)]")
	e.Linef("pub struct %s {", s.Name)
	e.Indent()

	for _, f := range s.Fields {
		emitFieldAnnotations(e, f)
		emitFieldDecl(e, f)
	}

	e.Dedent()
	e.Line("}")
}

// emitFieldAnnotations emits the attribute annotations for a field based on its modifier.
func emitFieldAnnotations(e *RustEmitter, f ast.Field) {
	switch f.Modifier {
	case ast.FieldInspect:
		e.Line("#[reflect(expand)]")
	case ast.FieldReactive:
		// Reactive fields are visible in the inspector like inspect fields.
		// Reactive tracking will be added in Task 14.
		e.Line("#[reflect(expand)]")
	case ast.FieldBare:
		e.Line("#[reflect(hidden)]")
		e.Line("#[visit(skip)]")
	case ast.FieldDerived:
		e.Line("#[reflect(hidden)]")
		e.Line("#[visit(skip)]")
	case ast.FieldNode:
		e.Line("#[reflect(expand)]")
	case ast.FieldNodes:
		e.Line("#[reflect(expand)]")
	case ast.FieldResource:
		e.Line("#[reflect(expand)]")
	}
}

// emitFieldDecl emits the field declaration line (with optional pub).
func emitFieldDecl(e *RustEmitter, f ast.Field) {
	vis := ""
	switch f.Modifier {
	case ast.FieldInspect, ast.FieldReactive, ast.FieldNode, ast.FieldNodes, ast.FieldResource:
		vis = "pub "
	}
	rustType := f.TypeExpr
	switch f.Modifier {
	case ast.FieldNode, ast.FieldNodes, ast.FieldResource:
		rustType = nodeFieldRustType(f)
	}
	e.Linef("%s%s: %s,", vis, f.Name, rustType)
}

// emitDefaultImpl emits a custom Default implementation for fields with default values.
func emitDefaultImpl(e *RustEmitter, s ast.Script) {
	e.Linef("impl Default for %s {", s.Name)
	e.Indent()
	e.Line("fn default() -> Self {")
	e.Indent()
	e.Line("Self {")
	e.Indent()

	for _, f := range s.Fields {
		if f.Default != "" && !isRuntimeResolved(f) {
			e.Linef("%s: %s,", f.Name, f.Default)
		}
	}
	e.Line("..Default::default()")

	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("}")
}

// emitScriptTraitImpl emits the ScriptTrait implementation with all handlers.
func emitScriptTraitImpl(e *RustEmitter, s ast.Script) {
	e.Linef("impl ScriptTrait for %s {", s.Name)
	e.Indent()

	needsOnStart := hasNodeOrResourceFields(s.Fields)
	emittedOnStart := false
	handlerCount := 0

	for _, h := range s.Handlers {
		if handlerCount > 0 {
			e.Blank()
		}
		if h.Kind == ast.HandlerStart && needsOnStart {
			// Merge generated resolution code with user body
			emitMergedOnStart(e, s.Fields, h.Body, s.Name)
			emittedOnStart = true
		} else {
			emitHandler(e, h, s.Name)
		}
		handlerCount++
	}

	// If we need on_start but user didn't write one, generate it
	if needsOnStart && !emittedOnStart {
		if handlerCount > 0 {
			e.Blank()
		}
		emitMergedOnStart(e, s.Fields, "", s.Name)
	}

	e.Dedent()
	e.Line("}")
}

// emitMergedOnStart emits an on_start handler with node/resource resolution code
// prepended before any user-provided body.
func emitMergedOnStart(e *RustEmitter, fields []ast.Field, userBody string, scriptName string) {
	e.Line("fn on_start(&mut self, ctx: &mut ScriptContext) {")
	e.Indent()

	resolution := GenerateNodeResolution(fields)
	emitHandlerBody(e, resolution)

	userBody = strings.TrimSpace(userBody)
	if userBody != "" {
		emitHandlerBody(e, userBody, scriptName)
	}

	e.Dedent()
	e.Line("}")
}

// emitHandler emits a single handler method within the ScriptTrait impl.
func emitHandler(e *RustEmitter, h ast.Handler, scriptName string) {
	switch h.Kind {
	case ast.HandlerInit:
		e.Line("fn on_init(&mut self, ctx: &mut ScriptContext) {")
		e.Indent()
		emitHandlerBody(e, h.Body, scriptName)
		e.Dedent()
		e.Line("}")

	case ast.HandlerStart:
		e.Line("fn on_start(&mut self, ctx: &mut ScriptContext) {")
		e.Indent()
		emitHandlerBody(e, h.Body, scriptName)
		e.Dedent()
		e.Line("}")

	case ast.HandlerUpdate:
		e.Line("fn on_update(&mut self, ctx: &mut ScriptContext) {")
		e.Indent()
		emitHandlerBody(e, h.Body, scriptName)
		e.Dedent()
		e.Line("}")

	case ast.HandlerDeinit:
		e.Line("fn on_deinit(&mut self, ctx: &mut ScriptDeinitContext) {")
		e.Indent()
		emitHandlerBody(e, h.Body, scriptName)
		e.Dedent()
		e.Line("}")

	case ast.HandlerEvent:
		emitEventHandler(e, h, scriptName)

	case ast.HandlerMessage:
		e.Line("fn on_message(&mut self, message: &mut dyn ScriptMessagePayload, ctx: &mut ScriptMessageContext) {")
		e.Indent()
		emitHandlerBody(e, h.Body, scriptName)
		e.Dedent()
		e.Line("}")
	}
}

// emitEventHandler emits an OS event handler with if-let matching on the event type.
func emitEventHandler(e *RustEmitter, h ast.Handler, scriptName string) {
	e.Line("fn on_os_event(&mut self, event: &Event<()>, ctx: &mut ScriptContext) {")
	e.Indent()

	// Look for a typed event parameter (e.g., "ev: KeyboardInput")
	var evParam *ast.Param
	for i := range h.Params {
		if h.Params[i].TypeExpr != "" && h.Params[i].Name != "ctx" {
			evParam = &h.Params[i]
			break
		}
	}

	if evParam != nil {
		e.Linef("if let Event::WindowEvent { event: WindowEvent::%s(%s), .. } = event {", evParam.TypeExpr, evParam.Name)
		e.Indent()
		emitHandlerBody(e, h.Body, scriptName)
		e.Dedent()
		e.Line("}")
	} else {
		emitHandlerBody(e, h.Body, scriptName)
	}

	e.Dedent()
	e.Line("}")
}

// emitHandlerBody emits the body of a handler, line by line.
// If scriptName is non-empty, FyroxScript shortcuts in the body are expanded
// to valid Rust via RewriteBody before emission.
func emitHandlerBody(e *RustEmitter, body string, scriptName ...string) {
	if len(scriptName) > 0 && scriptName[0] != "" {
		body = RewriteBody(body, scriptName[0])
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		e.Line(strings.TrimRight(line, " \t"))
	}
}
