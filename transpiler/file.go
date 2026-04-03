package transpiler

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
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
	var sections []string

	// 1. Rust passthrough items
	for _, item := range file.RustItems {
		src := strings.TrimSpace(item.Source)
		if src != "" {
			sections = append(sections, src)
		}
	}

	// 2. Signal message structs for all scripts
	for _, s := range file.Scripts {
		if len(s.Signals) > 0 {
			code := TranspileSignalStructs(s.Name, s.Signals)
			if code != "" {
				sections = append(sections, strings.TrimRight(code, "\n"))
			}
		}
	}

	// 3. Component structs (ECS)
	for _, c := range file.Components {
		code := TranspileComponent(c)
		if code != "" {
			sections = append(sections, strings.TrimRight(code, "\n"))
		}
	}

	// 4. Script structs + ScriptTrait impls
	for _, s := range file.Scripts {
		code := transpileScriptFull(s)
		if code != "" {
			sections = append(sections, strings.TrimRight(code, "\n"))
		}
	}

	// 5. System functions + runner (ECS)
	if len(file.Systems) > 0 {
		for _, s := range file.Systems {
			code := TranspileSystem(s)
			if code != "" {
				sections = append(sections, strings.TrimRight(code, "\n"))
			}
		}
		runner := TranspileSystemRunner(file.Systems)
		if runner != "" {
			sections = append(sections, strings.TrimRight(runner, "\n"))
		}
	}

	// 6. register_scripts function
	if len(file.Scripts) > 0 {
		reg := generateRegisterScripts(file.Scripts)
		sections = append(sections, strings.TrimRight(reg, "\n"))
	}

	return strings.Join(sections, "\n\n") + "\n"
}

// transpileScriptFull generates the complete Rust output for a single script,
// integrating reactive shadow fields, reactive update code, signal subscriptions,
// signal dispatch, and emit statement rewriting into the base TranspileScript output.
func transpileScriptFull(s ast.Script) string {
	// Augment the script with reactive shadow fields before transpiling.
	augmented := augmentWithReactive(s)

	// Augment handlers with signal/reactive integration.
	augmented = augmentHandlers(augmented, s)

	return TranspileScript(augmented)
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

	// Add defaults for shadow fields
	defaultInits := ReactiveDefaultInits(s.Fields)
	if len(defaultInits) > 0 {
		// Shadow fields with defaults need to be in the fields list with defaults set.
		// We already added them above; now set their defaults.
		for i, ex := range extras {
			for j := range result.Fields {
				if result.Fields[j].Name == ex.Name {
					// Parse the default init line to get the value.
					// ReactiveDefaultInits returns lines like "_health_prev: 100.0,"
					initLine := defaultInits[i]
					// Extract value after ": " and before trailing ","
					colonIdx := strings.Index(initLine, ": ")
					if colonIdx >= 0 {
						val := initLine[colonIdx+2:]
						val = strings.TrimSuffix(val, ",")
						result.Fields[j].Default = val
					}
					break
				}
			}
		}
	}

	return result
}

// augmentHandlers integrates reactive update code, signal subscriptions,
// signal dispatch, and emit rewriting into the script's handlers.
func augmentHandlers(s ast.Script, original ast.Script) ast.Script {
	result := s
	result.Handlers = make([]ast.Handler, len(s.Handlers))
	copy(result.Handlers, s.Handlers)

	// Reactive update code for on_update
	reactiveCode := GenerateReactiveUpdateCode(original.Fields, original.Watches)

	// Signal subscriptions for on_start
	subscriptionCode := TranspileConnectSubscriptions(original.Connects)

	// Signal dispatch for on_message
	dispatchCode := TranspileConnectDispatch(original.Connects)

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
	e.Line("pub fn register_scripts(ctx: &mut PluginRegistrationContext) {")
	e.Indent()
	for _, s := range scripts {
		e.Linef("ctx.serialization_context.script_constructors.add::<%s>(\"%s\");", s.Name, s.Name)
	}
	e.Dedent()
	e.Line("}")
	return e.String()
}

// TranspileRegisterScripts is a public convenience for generating just the
// register_scripts function, useful when building output incrementally.
func TranspileRegisterScripts(scripts []ast.Script) string {
	return fmt.Sprintf("%s", generateRegisterScripts(scripts))
}
