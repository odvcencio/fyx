package transpiler

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// TranspileScript takes an AST Script and returns the corresponding Rust source code.
// It generates a struct with derive macros, a Default impl (if any fields have defaults),
// and a ScriptTrait impl with lifecycle handlers.
func TranspileScript(s ast.Script) string {
	e := NewEmitter()
	EmitScript(e, s, Options{})
	return e.String()
}

// EmitScript writes a complete script definition into an emitter.
func EmitScript(e *RustEmitter, s ast.Script, opts Options) {
	emitStruct(e, s)
	e.Blank()
	emitDefaultImpl(e, s)
	e.Blank()
	emitScriptTraitImpl(e, s, opts)
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
	e.LineWithSource("#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]", s.Line)
	e.LineWithSource(fmt.Sprintf("#[type_uuid(id = \"%s\")]", scriptUUID(s.Name)), s.Line)
	e.LineWithSource("#[visit(optional)]", s.Line)
	e.LineWithSource(fmt.Sprintf("pub struct %s {", s.Name), s.Line)
	e.Indent()

	for _, f := range s.Fields {
		emitFieldAnnotations(e, f)
		emitFieldDecl(e, f)
	}

	e.Dedent()
	e.LineWithSource("}", s.Line)
}

// emitFieldAnnotations emits the attribute annotations for a field based on its modifier.
func emitFieldAnnotations(e *RustEmitter, f ast.Field) {
	switch f.Modifier {
	case ast.FieldInspect:
		e.LineWithSource("#[reflect(expand)]", f.Line)
	case ast.FieldReactive:
		// Reactive fields are visible in the inspector like inspect fields.
		// Reactive tracking will be added in Task 14.
		e.LineWithSource("#[reflect(expand)]", f.Line)
	case ast.FieldBare:
		e.LineWithSource("#[reflect(hidden)]", f.Line)
		e.LineWithSource("#[visit(skip)]", f.Line)
	case ast.FieldDerived:
		e.LineWithSource("#[reflect(hidden)]", f.Line)
		e.LineWithSource("#[visit(skip)]", f.Line)
	case ast.FieldNode:
		e.LineWithSource("#[reflect(expand)]", f.Line)
	case ast.FieldNodes:
		e.LineWithSource("#[reflect(expand)]", f.Line)
	case ast.FieldResource:
		e.LineWithSource("#[reflect(expand)]", f.Line)
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
	e.LineWithSource(fmt.Sprintf("%s%s: %s,", vis, f.Name, rustType), f.Line)
}

// emitDefaultImpl emits a custom Default implementation for every script field.
func emitDefaultImpl(e *RustEmitter, s ast.Script) {
	e.LineWithSource(fmt.Sprintf("impl Default for %s {", s.Name), s.Line)
	e.Indent()
	e.LineWithSource("fn default() -> Self {", s.Line)
	e.Indent()
	needsMutation := false
	for _, f := range s.Fields {
		if f.Modifier == ast.FieldReactive || f.Modifier == ast.FieldDerived {
			needsMutation = true
			break
		}
	}
	valueDecl := "let value = Self {"
	if needsMutation {
		valueDecl = "let mut value = Self {"
	}
	e.LineWithSource(valueDecl, s.Line)
	e.Indent()

	for _, f := range s.Fields {
		if isShadowField(f) || f.Modifier == ast.FieldDerived {
			e.LineWithSource(fmt.Sprintf("%s: Default::default(),", f.Name), f.Line)
			continue
		}
		if f.Default != "" && !isRuntimeResolved(f) {
			e.LineWithSource(fmt.Sprintf("%s: %s,", f.Name, f.Default), f.Line)
			continue
		}
		e.LineWithSource(fmt.Sprintf("%s: Default::default(),", f.Name), f.Line)
	}

	e.Dedent()
	e.LineWithSource("};", s.Line)

	for _, f := range s.Fields {
		if f.Modifier == ast.FieldDerived && f.Default != "" {
			e.LineWithSource(fmt.Sprintf("value.%s = %s;", f.Name, rewriteDefaultExpr(f.Default)), f.Line)
			e.LineWithSource(fmt.Sprintf("value.%s = value.%s.clone();", prevFieldName(f.Name), f.Name), f.Line)
		}
		if f.Modifier == ast.FieldReactive {
			e.LineWithSource(fmt.Sprintf("value.%s = value.%s.clone();", prevFieldName(f.Name), f.Name), f.Line)
		}
	}

	e.LineWithSource("value", s.Line)
	e.Dedent()
	e.LineWithSource("}", s.Line)
	e.Dedent()
	e.LineWithSource("}", s.Line)
}

func isShadowField(f ast.Field) bool {
	return strings.HasPrefix(f.Name, "_") && strings.HasSuffix(f.Name, "_prev")
}

func rewriteDefaultExpr(expr string) string {
	return strings.ReplaceAll(expr, "self.", "value.")
}

// emitScriptTraitImpl emits the ScriptTrait implementation with all handlers.
func emitScriptTraitImpl(e *RustEmitter, s ast.Script, opts Options) {
	e.LineWithSource(fmt.Sprintf("impl ScriptTrait for %s {", s.Name), s.Line)
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
			emitMergedOnStart(e, s.Fields, h, s.Name)
			emittedOnStart = true
		} else {
			emitHandler(e, h, s.Fields, s.Name, opts)
		}
		handlerCount++
	}

	// If we need on_start but user didn't write one, generate it
	if needsOnStart && !emittedOnStart {
		if handlerCount > 0 {
			e.Blank()
		}
		emitMergedOnStart(e, s.Fields, ast.Handler{Kind: ast.HandlerStart, Line: s.Line}, s.Name)
	}

	e.Dedent()
	e.LineWithSource("}", s.Line)
}

// emitMergedOnStart emits an on_start handler with node/resource resolution code
// prepended before any user-provided body.
func emitMergedOnStart(e *RustEmitter, fields []ast.Field, h ast.Handler, scriptName string) {
	line := h.Line
	if line == 0 {
		line = 1
	}
	e.LineWithSource("fn on_start(&mut self, ctx: &mut ScriptContext) {", line)
	e.Indent()

	for _, f := range fields {
		for _, line := range resolutionLinesForField(f) {
			e.LineWithSource(line, f.Line)
		}
	}

	userBody := strings.TrimSpace(h.Body)
	if userBody != "" {
		emitHandlerBody(e, userBody, h.BodyLine, fields, h.Kind, scriptName)
	}

	e.Dedent()
	e.LineWithSource("}", line)
}

// emitHandler emits a single handler method within the ScriptTrait impl.
func emitHandler(e *RustEmitter, h ast.Handler, fields []ast.Field, scriptName string, opts Options) {
	switch h.Kind {
	case ast.HandlerInit:
		e.LineWithSource("fn on_init(&mut self, ctx: &mut ScriptContext) {", h.Line)
		e.Indent()
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
		e.Dedent()
		e.LineWithSource("}", h.Line)

	case ast.HandlerStart:
		e.LineWithSource("fn on_start(&mut self, ctx: &mut ScriptContext) {", h.Line)
		e.Indent()
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
		e.Dedent()
		e.LineWithSource("}", h.Line)

	case ast.HandlerUpdate:
		e.LineWithSource("fn on_update(&mut self, ctx: &mut ScriptContext) {", h.Line)
		e.Indent()
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
		e.Dedent()
		e.LineWithSource("}", h.Line)

	case ast.HandlerDeinit:
		e.LineWithSource("fn on_deinit(&mut self, ctx: &mut ScriptDeinitContext) {", h.Line)
		e.Indent()
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
		e.Dedent()
		e.LineWithSource("}", h.Line)

	case ast.HandlerEvent:
		emitEventHandler(e, h, fields, scriptName)

	case ast.HandlerMessage:
		e.LineWithSource("fn on_message(&mut self, message: &mut dyn ScriptMessagePayload, ctx: &mut ScriptMessageContext) {", h.Line)
		e.Indent()
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
		e.Dedent()
		e.LineWithSource("}", h.Line)
	}
}

// emitEventHandler emits an OS event handler with if-let matching on the event type.
func emitEventHandler(e *RustEmitter, h ast.Handler, fields []ast.Field, scriptName string) {
	e.LineWithSource("fn on_os_event(&mut self, event: &Event<()>, ctx: &mut ScriptContext) {", h.Line)
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
		e.LineWithSource(fmt.Sprintf("if let Event::WindowEvent { event: WindowEvent::%s(%s), .. } = event {", evParam.TypeExpr, evParam.Name), h.Line)
		e.Indent()
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
		e.Dedent()
		e.LineWithSource("}", h.Line)
	} else {
		emitHandlerBody(e, h.Body, h.BodyLine, fields, h.Kind, scriptName)
	}

	e.Dedent()
	e.LineWithSource("}", h.Line)
}

// emitHandlerBody emits the body of a handler, line by line, with source mapping.
func emitHandlerBody(e *RustEmitter, body string, sourceLine int, fields []ast.Field, kind ast.HandlerKind, scriptName string) {
	if scriptName != "" {
		body = RewriteBody(body, scriptName, fields, kind)
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		e.LineWithSource(strings.TrimRight(line, " \t"), sourceLine+i)
	}
}
