package transpiler

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// TranspileFile orchestrates all transpiler components and produces a complete .rs file
// from a parsed AST File. The output is structured in this order:
//  1. Rust items (passthrough — use, fn, struct, etc.)
//  2. Signal message structs (from all scripts' signal declarations)
//  3. Component structs (ECS)
//  4. Script structs + ScriptTrait impls (with reactive, signal, and body rewriting integrated)
//  5. System functions + runner (ECS)
//  6. register_scripts function
func TranspileFile(file ast.File) string {
	return TranspileFileResult(file, Options{}).Code
}

// TranspileFileResult transpiles a file with project-aware options and source mapping.
func TranspileFileResult(file ast.File, opts Options) GeneratedFile {
	e := NewEmitter()
	EmitFile(e, file, opts)
	return GeneratedFile{
		Code:          e.String(),
		LineMap:       e.LineMap(),
		ArbiterBundle: RenderArbiterBundle(file.ArbiterDecls),
	}
}

// EmitFile writes a complete file into an emitter.
func EmitFile(e *RustEmitter, file ast.File, opts Options) {
	if len(opts.SignalIndex) == 0 {
		opts.SignalIndex = BuildSignalIndex([]ast.File{file})
	}

	hasSection := false
	emitSectionBreak := func() {
		if hasSection {
			e.Blank()
			e.Blank()
		}
		hasSection = true
	}

	if len(file.Imports) > 0 {
		emitSectionBreak()
		for _, imp := range file.Imports {
			usePath := relativeImportUsePath(opts.CurrentModule, importSegments(imp.Path))
			e.LineWithSource(fmt.Sprintf("use %s::*;", usePath), imp.Line)
		}
	}

	for _, item := range file.RustItems {
		src := strings.TrimSpace(item.Source)
		if src == "" {
			continue
		}
		emitSectionBreak()
		for i, line := range strings.Split(src, "\n") {
			e.LineWithSource(strings.TrimRight(line, " \t"), item.Line+i)
		}
	}

	if len(file.ArbiterDecls) > 0 {
		emitSectionBreak()
		EmitArbiterBundle(e, file.ArbiterDecls)
	}

	for _, s := range file.Scripts {
		if len(s.Signals) == 0 {
			continue
		}
		emitSectionBreak()
		EmitSignalStructs(e, s.Name, s.Signals)
	}

	for _, c := range file.Components {
		emitSectionBreak()
		EmitComponent(e, c)
	}

	for _, s := range file.Scripts {
		emitSectionBreak()
		EmitScript(e, transpileScriptFull(s, opts), opts)
	}

	if len(file.Systems) > 0 {
		for _, s := range file.Systems {
			emitSectionBreak()
			EmitSystem(e, s)
		}
		emitSectionBreak()
		EmitSystemRunner(e, file.Systems)
	}

	if len(file.Scripts) > 0 {
		emitSectionBreak()
		EmitRegisterScripts(e, file.Scripts)
	}
}

// transpileScriptFull generates the complete Rust output for a single script,
// integrating reactive shadow fields, reactive update code, signal subscriptions,
// signal dispatch, and emit statement rewriting into the base TranspileScript output.
func transpileScriptFull(s ast.Script, opts Options) ast.Script {
	// Augment the script with reactive shadow fields before transpiling.
	augmented := augmentWithReactive(s)

	// Augment handlers with signal/reactive integration.
	return augmentHandlers(augmented, s, opts)
}

// augmentWithReactive adds reactive shadow field declarations and default inits
// to the script's field list so they appear in the struct and Default impl.
func augmentWithReactive(s ast.Script) ast.Script {
	extras := ReactiveFieldDecls(s.Fields)
	if len(extras) == 0 {
		return s
	}

	result := s
	result.Fields = make([]ast.Field, len(s.Fields))
	copy(result.Fields, s.Fields)

	for _, ex := range extras {
		result.Fields = append(result.Fields, ast.Field{
			Modifier: ast.FieldBare,
			Name:     ex.Name,
			TypeExpr: ex.TypeExpr,
		})
	}

	return result
}

// augmentHandlers integrates reactive update code, signal subscriptions,
// signal dispatch, and emit rewriting into the script's handlers.
func augmentHandlers(s ast.Script, original ast.Script, opts Options) ast.Script {
	result := s
	result.Handlers = make([]ast.Handler, len(s.Handlers))
	copy(result.Handlers, s.Handlers)

	// Reactive update code for on_update
	reactiveCode := GenerateReactiveUpdateCode(original.Fields, original.Watches)

	// Signal subscriptions for on_start
	subscriptionCode := TranspileConnectSubscriptions(original.Connects)

	// Signal dispatch for on_message
	dispatchCode := TranspileConnectDispatch(original.Connects, opts.SignalIndex)

	// Rewrite emit statements in all handler bodies
	if len(original.Signals) > 0 {
		for i := range result.Handlers {
			result.Handlers[i].Body = RewriteEmitStatements(
				result.Handlers[i].Body,
				original.Name,
				original.Signals,
			)
		}
	}

	// Integrate reactive code into on_update
	if reactiveCode != "" {
		found := false
		for i, h := range result.Handlers {
			if h.Kind == ast.HandlerUpdate {
				result.Handlers[i].Body = mergeBodyCode(h.Body, reactiveCode)
				found = true
				break
			}
		}
		if !found {
			result.Handlers = append(result.Handlers, ast.Handler{
				Kind: ast.HandlerUpdate,
				Body: reactiveCode,
			})
		}
	}

	// Integrate signal subscriptions into on_start
	if subscriptionCode != "" {
		found := false
		for i, h := range result.Handlers {
			if h.Kind == ast.HandlerStart {
				result.Handlers[i].Body = mergeBodyCode(subscriptionCode, h.Body)
				found = true
				break
			}
		}
		if !found {
			result.Handlers = append(result.Handlers, ast.Handler{
				Kind: ast.HandlerStart,
				Body: subscriptionCode,
			})
		}
	}

	// Integrate signal dispatch into on_message
	if dispatchCode != "" {
		found := false
		for i, h := range result.Handlers {
			if h.Kind == ast.HandlerMessage {
				result.Handlers[i].Body = mergeBodyCode(h.Body, dispatchCode)
				found = true
				break
			}
		}
		if !found {
			result.Handlers = append(result.Handlers, ast.Handler{
				Kind: ast.HandlerMessage,
				Body: dispatchCode,
			})
		}
	}

	return result
}

// mergeBodyCode combines two code blocks with a blank line separator.
// Either block can be empty.
func mergeBodyCode(first, second string) string {
	first = strings.TrimSpace(first)
	second = strings.TrimSpace(second)
	if first == "" {
		return second
	}
	if second == "" {
		return first
	}
	return first + "\n\n" + second
}

// generateRegisterScripts generates the register_scripts function that registers
// all script constructors with the Fyrox plugin system.
func generateRegisterScripts(scripts []ast.Script) string {
	e := NewEmitter()
	EmitRegisterScripts(e, scripts)
	return e.String()
}

// EmitRegisterScripts writes the register_scripts helper into an emitter.
func EmitRegisterScripts(e *RustEmitter, scripts []ast.Script) {
	line := 1
	if len(scripts) > 0 && scripts[0].Line != 0 {
		line = scripts[0].Line
	}
	e.LineWithSource("pub fn register_scripts(ctx: &mut PluginRegistrationContext) {", line)
	e.Indent()
	for _, s := range scripts {
		e.LineWithSource(fmt.Sprintf("ctx.serialization_context.script_constructors.add::<%s>(\"%s\");", s.Name, s.Name), s.Line)
	}
	e.Dedent()
	e.LineWithSource("}", line)
}

// TranspileRegisterScripts is a public convenience for generating just the
// register_scripts function, useful when building output incrementally.
func TranspileRegisterScripts(scripts []ast.Script) string {
	return fmt.Sprintf("%s", generateRegisterScripts(scripts))
}
