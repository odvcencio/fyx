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
	if len(opts.ComponentHandleIndex) == 0 {
		opts.ComponentHandleIndex = BuildComponentHandleIndex([]ast.File{file})
	}

	hasSection := false
	emitSectionBreak := func() {
		if hasSection {
			e.Blank()
			e.Blank()
		}
		hasSection = true
	}

	if needsGeneratedPrelude(file) {
		emitSectionBreak()
		needSceneLifetime := needsSceneLifetimeSupport(file)
		needEntityLifetime := fileNeedsEntityLifetimeSupport(file)
		needNodeHelpers := needsNodePathHelpers(file) || needSceneLifetime
		for _, line := range generatedPreludeLines(needNodeHelpers) {
			e.LineWithSource(line, generatedPreludeLine(file))
		}
		helperLines := generatedGameplayHelperLines(needSceneLifetime, needEntityLifetime)
		if len(helperLines) > 0 {
			e.Blank()
			for _, line := range helperLines {
				e.LineWithSource(line, generatedPreludeLine(file))
			}
		}
		if needNodeHelpers {
			e.Blank()
			for _, line := range generatedNodePathHelperLines() {
				e.LineWithSource(line, generatedPreludeLine(file))
			}
		}
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

	needEntityLifetime := fileNeedsEntityLifetimeSupport(file)
	if len(file.Systems) > 0 || needEntityLifetime {
		for _, s := range file.Systems {
			emitSectionBreak()
			EmitSystem(e, s, opts)
		}
		emitSectionBreak()
		emitSystemRunnerInternal(e, file.Systems, needEntityLifetime)
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
	// Augment the script with hidden runtime fields before transpiling.
	augmented := augmentWithGameplayRuntimeFields(s)

	// Augment handlers with signal/reactive integration.
	return augmentHandlers(augmented, s, opts)
}

// augmentHandlers integrates reactive update code, signal subscriptions,
// signal dispatch, and emit rewriting into the script's handlers.
func augmentHandlers(s ast.Script, original ast.Script, opts Options) ast.Script {
	result := s
	result.Handlers = make([]ast.Handler, len(s.Handlers))
	copy(result.Handlers, s.Handlers)

	timerCode := GenerateTimerUpdateCode(original.Fields)
	sceneLifetimeUpdateCode := GenerateSceneLifetimeUpdateCode(result.Fields)
	sceneLifetimeDeinitCode := GenerateSceneLifetimeDeinitCode(result.Fields)
	stateStartCode := GenerateStateStartCode(original.Name, original.States, result.Fields, opts.SignalIndex)
	stateUpdateCode := GenerateStateUpdateCode(original.Name, original.States, result.Fields, opts.SignalIndex)
	stateDeinitCode := GenerateStateDeinitCode(original.Name, original.States, result.Fields, opts.SignalIndex)

	// Reactive update code for on_update
	reactiveCode := GenerateReactiveUpdateCode(original.Fields, original.Watches)

	// Signal subscriptions for on_start
	subscriptionCode := TranspileConnectSubscriptions(original.Connects)

	// Signal dispatch for on_message
	dispatchCode := TranspileConnectDispatch(original.Connects, original.Name, result.Fields, original.States, opts.SignalIndex)

	// Rewrite emit statements in all handler bodies using the project signal index
	for i := range result.Handlers {
		result.Handlers[i].Body = RewriteEmitStatementsWithOptions(
			result.Handlers[i].Body,
			original.Name,
			EmitRewriteOptions{
				SignalIndex:    opts.SignalIndex,
				HandleBindings: analyzeScriptHandleBindings(result.Handlers[i].Body, original.Fields, result.Handlers[i].Kind),
			},
		)
	}

	// Integrate reactive code into on_update
	updatePrefix := mergeBodyCode(timerCode, sceneLifetimeUpdateCode)
	updateSuffix := reactiveCode
	if updatePrefix != "" || updateSuffix != "" {
		found := false
		for i, h := range result.Handlers {
			if h.Kind == ast.HandlerUpdate {
				result.Handlers[i].Body = prependHandlerCode(h.Body, updatePrefix)
				result.Handlers[i].Body = appendHandlerCode(result.Handlers[i].Body, stateUpdateCode)
				result.Handlers[i].Body = appendHandlerCode(result.Handlers[i].Body, updateSuffix)
				found = true
				break
			}
		}
		if !found {
			result.Handlers = append(result.Handlers, ast.Handler{
				Kind: ast.HandlerUpdate,
				Body: mergeBodyCode(mergeBodyCode(updatePrefix, stateUpdateCode), updateSuffix),
			})
		}
	} else if stateUpdateCode != "" {
		result.Handlers = append(result.Handlers, ast.Handler{
			Kind: ast.HandlerUpdate,
			Body: stateUpdateCode,
		})
	}

	// Integrate signal subscriptions into on_start
	if subscriptionCode != "" || stateStartCode != "" {
		found := false
		for i, h := range result.Handlers {
			if h.Kind == ast.HandlerStart {
				result.Handlers[i].Body = mergeBodyCode(subscriptionCode, h.Body)
				result.Handlers[i].Body = mergeBodyCode(result.Handlers[i].Body, stateStartCode)
				found = true
				break
			}
		}
		if !found {
			result.Handlers = append(result.Handlers, ast.Handler{
				Kind: ast.HandlerStart,
				Body: mergeBodyCode(subscriptionCode, stateStartCode),
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

	if stateDeinitCode != "" || sceneLifetimeDeinitCode != "" {
		found := false
		for i, h := range result.Handlers {
			if h.Kind == ast.HandlerDeinit {
				result.Handlers[i].Body = appendHandlerCode(h.Body, stateDeinitCode)
				result.Handlers[i].Body = appendHandlerCode(result.Handlers[i].Body, sceneLifetimeDeinitCode)
				found = true
				break
			}
		}
		if !found {
			result.Handlers = append(result.Handlers, ast.Handler{
				Kind: ast.HandlerDeinit,
				Body: mergeBodyCode(stateDeinitCode, sceneLifetimeDeinitCode),
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

func needsGeneratedPrelude(file ast.File) bool {
	return len(file.Scripts) > 0 || len(file.Systems) > 0
}

func needsNodePathHelpers(file ast.File) bool {
	for _, script := range file.Scripts {
		for _, field := range script.Fields {
			switch field.Modifier {
			case ast.FieldNode, ast.FieldNodes:
				return true
			}
		}
	}
	return false
}

func needsSceneLifetimeSupport(file ast.File) bool {
	for _, script := range file.Scripts {
		if scriptUsesSceneLifetimeSupport(script) {
			return true
		}
	}
	return false
}

func generatedPreludeLine(file ast.File) int {
	for _, line := range []int{
		firstImportLine(file),
		firstRustItemLine(file),
		firstScriptLine(file),
		firstSystemLine(file),
	} {
		if line != 0 {
			return line
		}
	}
	return 1
}

func firstImportLine(file ast.File) int {
	if len(file.Imports) == 0 {
		return 0
	}
	return file.Imports[0].Line
}

func firstRustItemLine(file ast.File) int {
	if len(file.RustItems) == 0 {
		return 0
	}
	return file.RustItems[0].Line
}

func firstScriptLine(file ast.File) int {
	if len(file.Scripts) == 0 {
		return 0
	}
	return file.Scripts[0].Line
}

func firstSystemLine(file ast.File) int {
	if len(file.Systems) == 0 {
		return 0
	}
	return file.Systems[0].Line
}

func generatedPreludeLines(includeNodeGraphImports bool) []string {
	useLines := []string{
		"use fyrox::asset::Resource;",
		"use fyrox::core::pool::Handle;",
		"use fyrox::core::reflect::prelude::*;",
		"use fyrox::core::type_traits::prelude::*;",
		"use fyrox::core::visitor::prelude::*;",
		"use fyrox::event::{Event, WindowEvent};",
		"use fyrox::plugin::error::GameResult;",
		"use fyrox::plugin::{PluginContext, PluginRegistrationContext};",
		"use fyrox::resource::model::Model;",
		"use fyrox::scene::node::Node;",
		"use fyrox::script::{ScriptContext, ScriptDeinitContext, ScriptMessageContext, ScriptMessagePayload, ScriptTrait};",
	}
	if includeNodeGraphImports {
		useLines = append(useLines,
			"use fyrox::graph::SceneGraph;",
			"use fyrox::scene::graph::Graph;",
		)
	}
	var lines []string
	for _, useLine := range useLines {
		lines = append(lines, "#[allow(unused_imports)]")
		lines = append(lines, useLine)
	}
	return lines
}

func generatedNodePathHelperLines() []string {
	return []string{
		"fn fyx_find_relative_node_path(graph: &Graph, root: Handle<Node>, path: &str) -> Handle<Node> {",
		"    let parts = path",
		"        .split('/')",
		"        .filter(|segment| !segment.is_empty())",
		"        .collect::<Vec<_>>();",
		"    if parts.is_empty() {",
		"        panic!(\"Fyx relative node path is empty\");",
		"    };",
		"    let mut current = root;",
		"    for segment in parts {",
		"        match segment {",
		"            \".\" => {}",
		"            \"..\" => {",
		"                current = graph",
		"                    .try_get_node(current)",
		"                    .map(|node| node.parent())",
		"                    .unwrap_or_else(|_| panic!(\"Fyx relative node path not found: {}\", path));",
		"            }",
		"            name => {",
		"                let current_node = graph",
		"                    .try_get_node(current)",
		"                    .unwrap_or_else(|_| panic!(\"Fyx relative node path not found: {}\", path));",
		"                let Some(next) = current_node.children().iter().copied().find(|child| {",
		"                    graph",
		"                        .try_get_node(*child)",
		"                        .map(|node| node.name() == name)",
		"                        .unwrap_or(false)",
		"                }) else {",
		"                    panic!(\"Fyx relative node path not found: {}\", path);",
		"                };",
		"                current = next;",
		"            }",
		"        }",
		"    }",
		"    current",
		"}",
		"",
		"fn fyx_find_relative_nodes_path(graph: &Graph, root: Handle<Node>, pattern: &str) -> Vec<Handle<Node>> {",
		"    if pattern == \"*\" {",
		"        return graph",
		"            .try_get_node(root)",
		"            .map(|node| node.children().to_vec())",
		"            .unwrap_or_else(|_| panic!(\"Fyx relative node path not found: {}\", pattern));",
		"    }",
		"    if let Some(parent_path) = pattern.strip_suffix(\"/*\") {",
		"        let parent = if parent_path.is_empty() || parent_path == \".\" {",
		"            root",
		"        } else {",
		"            fyx_find_relative_node_path(graph, root, parent_path)",
		"        };",
		"        return graph",
		"            .try_get_node(parent)",
		"            .map(|node| node.children().to_vec())",
		"            .unwrap_or_else(|_| panic!(\"Fyx relative node path not found: {}\", pattern));",
		"    }",
		"    vec![fyx_find_relative_node_path(graph, root, pattern)]",
		"}",
		"",
		"fn fyx_find_node_path(graph: &Graph, path: &str) -> Handle<Node> {",
		"    let parts = path",
		"        .split('/')",
		"        .filter(|segment| !segment.is_empty())",
		"        .collect::<Vec<_>>();",
		"    if parts.is_empty() {",
		"        panic!(\"Fyx node path is empty\");",
		"    };",
		"    if parts.len() == 1 {",
		"        return graph",
		"            .find_by_name_from_root(parts[0])",
		"            .map(|(handle, _)| handle)",
		"            .unwrap_or_else(|| panic!(\"Fyx node path not found: {}\", path));",
		"    }",
		"    let Some((mut current, _)) = graph.find_by_name_from_root(parts[0]) else {",
		"        panic!(\"Fyx node path not found: {}\", path);",
		"    };",
		"    for segment in parts.iter().skip(1) {",
		"        let current_node = graph",
		"            .try_get_node(current)",
		"            .unwrap_or_else(|_| panic!(\"Fyx node path not found: {}\", path));",
		"        let Some(next) = current_node.children().iter().copied().find(|child| {",
		"            graph",
		"                .try_get_node(*child)",
		"                .map(|node| node.name() == *segment)",
		"                .unwrap_or(false)",
		"        }) else {",
		"            panic!(\"Fyx node path not found: {}\", path);",
		"        };",
		"        current = next;",
		"    }",
		"    current",
		"}",
		"",
		"fn fyx_expect_node_type<T>(graph: &Graph, handle: Handle<Node>, path: &str, expected_type: &str) -> Handle<Node> {",
		"    if graph.try_get_of_type::<T>(handle).is_err() {",
		"        panic!(\"Fyx node path '{}' did not resolve to expected type {}\", path, expected_type);",
		"    }",
		"    handle",
		"}",
		"",
		"fn fyx_expect_nodes_type<T>(graph: &Graph, handles: Vec<Handle<Node>>, path: &str, expected_type: &str) -> Vec<Handle<Node>> {",
		"    for handle in &handles {",
		"        if graph.try_get_of_type::<T>(*handle).is_err() {",
		"            panic!(\"Fyx node path '{}' did not resolve to expected type {}\", path, expected_type);",
		"        }",
		"    }",
		"    handles",
		"}",
		"",
		"fn fyx_find_nodes_path(graph: &Graph, pattern: &str) -> Vec<Handle<Node>> {",
		"    if let Some(parent_path) = pattern.strip_suffix(\"/*\") {",
		"        if parent_path.is_empty() {",
		"            return graph",
		"                .try_get_node(graph.root())",
		"                .map(|node| node.children().to_vec())",
		"                .unwrap_or_else(|_| panic!(\"Fyx node path not found: {}\", pattern));",
		"        }",
		"        let parent = fyx_find_node_path(graph, parent_path);",
		"        return graph",
		"            .try_get_node(parent)",
		"            .map(|node| node.children().to_vec())",
		"            .unwrap_or_else(|_| panic!(\"Fyx node path not found: {}\", pattern));",
		"    }",
		"    vec![fyx_find_node_path(graph, pattern)]",
		"}",
	}
}
