package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

const (
	stateFieldName           = "_fyx_state"
	stateTransitionFieldName = "_fyx_transition"
	stateTransitionLoopLimit = 8
)

var stateTransitionRe = regexp.MustCompile(`(^|[^[:alnum:]_])go\s+([A-Za-z_][A-Za-z0-9_]*)\s*;?`)

func stateEnumName(scriptName string) string {
	return toPascalCase(scriptName) + "State"
}

func stateVariantName(name string) string {
	return toPascalCase(name)
}

func hasStateMachine(states []ast.State) bool {
	return len(states) > 0
}

func initialStateExpr(scriptName string, states []ast.State) string {
	if len(states) == 0 {
		return ""
	}
	return stateEnumName(scriptName) + "::" + stateVariantName(states[0].Name)
}

func emitStateEnum(e *RustEmitter, s ast.Script) {
	if !hasStateMachine(s.States) {
		return
	}

	e.LineWithSource("#[derive(Visit, Reflect, Debug, Clone, Copy, PartialEq, Eq, Default)]", s.Line)
	e.LineWithSource(fmt.Sprintf("enum %s {", stateEnumName(s.Name)), s.Line)
	e.Indent()
	for i, state := range s.States {
		if i == 0 {
			e.LineWithSource("#[default]", state.Line)
		}
		e.LineWithSource(fmt.Sprintf("%s,", stateVariantName(state.Name)), state.Line)
	}
	e.Dedent()
	e.LineWithSource("}", s.Line)
}

func RewriteStateTransitions(body, scriptName string, states []ast.State) string {
	if !hasStateMachine(states) {
		return body
	}

	validStates := make(map[string]struct{}, len(states))
	for _, state := range states {
		validStates[state.Name] = struct{}{}
	}

	return stateTransitionRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := stateTransitionRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		leading, target := parts[1], parts[2]
		if _, ok := validStates[target]; !ok {
			return match
		}
		return leading + "self." + stateTransitionFieldName + " = Some(" + stateEnumName(scriptName) + "::" + stateVariantName(target) + ");"
	})
}

func stateHandlerBody(state ast.State, kind ast.StateHandlerKind) (ast.StateHandler, bool) {
	for _, handler := range state.Handlers {
		if handler.Kind == kind {
			return handler, true
		}
	}
	return ast.StateHandler{}, false
}

func transpileStateHandlerBody(scriptName string, fields []ast.Field, states []ast.State, body string, line int, signalIndex SignalIndex, kind ast.HandlerKind) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	body = RewriteEmitStatementsWithOptions(body, scriptName, EmitRewriteOptions{
		SignalIndex:    signalIndex,
		HandleBindings: analyzeScriptHandleBindings(body, fields, kind),
	})
	body = RewriteBodyWithStates(body, scriptName, fields, states, kind)
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	e := NewEmitter()
	for i, lineText := range strings.Split(body, "\n") {
		e.LineWithSource(strings.TrimRight(lineText, " \t"), line+i)
	}
	return strings.TrimRight(e.String(), "\n")
}

func emitStateDispatchMatch(e *RustEmitter, scriptName string, states []ast.State, fields []ast.Field, signalIndex SignalIndex, stateKind ast.StateHandlerKind, handlerKind ast.HandlerKind) bool {
	hasBodies := false
	for _, state := range states {
		if handler, ok := stateHandlerBody(state, stateKind); ok && strings.TrimSpace(handler.Body) != "" {
			hasBodies = true
			break
		}
	}
	if !hasBodies {
		return false
	}

	e.Linef("match self.%s {", stateFieldName)
	e.Indent()
	for _, state := range states {
		e.Linef("%s::%s => {", stateEnumName(scriptName), stateVariantName(state.Name))
		e.Indent()
		if handler, ok := stateHandlerBody(state, stateKind); ok {
			body := transpileStateHandlerBody(scriptName, fields, states, handler.Body, handler.BodyLine, signalIndex, handlerKind)
			if body != "" {
				for _, line := range strings.Split(body, "\n") {
					e.Line(strings.TrimRight(line, " \t"))
				}
			}
		}
		e.Dedent()
		e.Line("}")
	}
	e.Dedent()
	e.Line("}")
	return true
}

func GenerateStateStartCode(scriptName string, states []ast.State, fields []ast.Field, signalIndex SignalIndex) string {
	if !hasStateMachine(states) {
		return ""
	}

	e := NewEmitter()
	emitted := emitStateDispatchMatch(e, scriptName, states, fields, signalIndex, ast.StateHandlerEnter, ast.HandlerStart)
	transitionLoop := GenerateStateTransitionLoopCode(scriptName, states, fields, signalIndex, ast.HandlerStart)
	if transitionLoop != "" {
		if emitted {
			e.Blank()
		}
		for _, line := range strings.Split(transitionLoop, "\n") {
			e.Line(strings.TrimRight(line, " \t"))
		}
		emitted = true
	}
	if !emitted {
		return ""
	}
	return strings.TrimRight(e.String(), "\n")
}

func GenerateStateUpdateCode(scriptName string, states []ast.State, fields []ast.Field, signalIndex SignalIndex) string {
	if !hasStateMachine(states) {
		return ""
	}

	e := NewEmitter()
	emitted := emitStateDispatchMatch(e, scriptName, states, fields, signalIndex, ast.StateHandlerUpdate, ast.HandlerUpdate)
	transitionLoop := GenerateStateTransitionLoopCode(scriptName, states, fields, signalIndex, ast.HandlerUpdate)
	if transitionLoop != "" {
		if emitted {
			e.Blank()
		}
		for _, line := range strings.Split(transitionLoop, "\n") {
			e.Line(strings.TrimRight(line, " \t"))
		}
		emitted = true
	}
	if !emitted {
		return ""
	}
	return strings.TrimRight(e.String(), "\n")
}

func GenerateStateDeinitCode(scriptName string, states []ast.State, fields []ast.Field, signalIndex SignalIndex) string {
	if !hasStateMachine(states) {
		return ""
	}

	e := NewEmitter()
	if !emitStateDispatchMatch(e, scriptName, states, fields, signalIndex, ast.StateHandlerExit, ast.HandlerDeinit) {
		return ""
	}
	return strings.TrimRight(e.String(), "\n")
}

func GenerateStateTransitionLoopCode(scriptName string, states []ast.State, fields []ast.Field, signalIndex SignalIndex, kind ast.HandlerKind) string {
	if !hasStateMachine(states) {
		return ""
	}

	hasEnter := false
	hasExit := false
	for _, state := range states {
		if handler, ok := stateHandlerBody(state, ast.StateHandlerEnter); ok && strings.TrimSpace(handler.Body) != "" {
			hasEnter = true
		}
		if handler, ok := stateHandlerBody(state, ast.StateHandlerExit); ok && strings.TrimSpace(handler.Body) != "" {
			hasExit = true
		}
	}

	e := NewEmitter()
	e.Linef("for _ in 0..%d {", stateTransitionLoopLimit)
	e.Indent()
	e.Linef("let Some(next_state) = self.%s.take() else {", stateTransitionFieldName)
	e.Indent()
	e.Line("break;")
	e.Dedent()
	e.Line("};")
	e.Linef("if self.%s == next_state {", stateFieldName)
	e.Indent()
	e.Line("continue;")
	e.Dedent()
	e.Line("}")

	if hasExit {
		e.Blank()
		emitStateDispatchMatch(e, scriptName, states, fields, signalIndex, ast.StateHandlerExit, kind)
	}

	e.Blank()
	e.Linef("self.%s = next_state;", stateFieldName)

	if hasEnter {
		e.Blank()
		emitStateDispatchMatch(e, scriptName, states, fields, signalIndex, ast.StateHandlerEnter, kind)
	}

	e.Dedent()
	e.Line("}")
	return strings.TrimRight(e.String(), "\n")
}
