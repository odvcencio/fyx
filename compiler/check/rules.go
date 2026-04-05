package check

import (
	"regexp"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

func (c *checkCtx) checkDuplicateFields(script ast.Script) {
	seen := map[string]int{}
	for _, field := range script.Fields {
		if _, ok := seen[field.Name]; ok {
			c.add(duplicateField(field.Name, script.Name, c.span(field.Line)))
		}
		seen[field.Name] = field.Line

		if field.TypeExpr == "" && field.Modifier != ast.FieldDerived {
			c.add(missingFieldType(field.Name, c.span(field.Line)))
		}

		if (field.Modifier == ast.FieldNode || field.Modifier == ast.FieldNodes) && field.Default != "" {
			if !strings.HasPrefix(field.Default, "\"") {
				c.add(nodePathWithoutQuotes(field.Name, c.span(field.Line)))
			}
		}
	}
}

// --- Task 4: Handler and State rules ---

var goStateRe = regexp.MustCompile(`\bgo\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)
var bareDtRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])dt\b`)

func (c *checkCtx) checkHandlers(script ast.Script) {
	for _, h := range script.Handlers {
		if h.Kind != ast.HandlerUpdate && bareDtRe.MatchString(h.Body) {
			c.add(dtOutsideUpdate(handlerKindName(h.Kind), c.span(h.BodyLine)))
		}
	}
}

func (c *checkCtx) checkStates(script ast.Script) {
	stateNames := make([]string, len(script.States))
	stateSet := make(map[string]bool, len(script.States))
	for i, state := range script.States {
		stateNames[i] = state.Name
		stateSet[state.Name] = true
	}
	for _, state := range script.States {
		for _, handler := range state.Handlers {
			for _, match := range goStateRe.FindAllStringSubmatch(handler.Body, -1) {
				target := match[1]
				if !stateSet[target] {
					c.add(goToUnknownState(target, script.Name, stateNames, c.span(handler.BodyLine)))
				}
			}
		}
	}
}

func handlerKindName(k ast.HandlerKind) string {
	switch k {
	case ast.HandlerInit:
		return "init"
	case ast.HandlerStart:
		return "start"
	case ast.HandlerUpdate:
		return "update"
	case ast.HandlerDeinit:
		return "deinit"
	case ast.HandlerEvent:
		return "event"
	case ast.HandlerMessage:
		return "message"
	default:
		return "unknown"
	}
}

// --- Task 5: Signal rules ---

var emitPrefixCheckRe = regexp.MustCompile(`emit\s+([A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)?)\(`)

func (c *checkCtx) checkConnects(script ast.Script) {
	if c.opts.SignalIndex == nil {
		return
	}
	for _, conn := range script.Connects {
		key := conn.ScriptName + "::" + conn.SignalName
		if _, ok := c.opts.SignalIndex[key]; !ok {
			c.add(signalNotFound(conn.ScriptName, conn.SignalName, c.span(conn.Line)))
		}
	}
}

func (c *checkCtx) checkEmitArgCounts(script ast.Script) {
	idx := c.opts.SignalIndex
	if len(idx) == 0 {
		idx = make(SignalIndex)
		for _, sig := range script.Signals {
			idx[script.Name+"::"+sig.Name] = sig.Params
		}
	}
	for _, h := range script.Handlers {
		c.checkEmitInBody(h.Body, script.Name, idx, h.BodyLine)
	}
	for _, state := range script.States {
		for _, sh := range state.Handlers {
			c.checkEmitInBody(sh.Body, script.Name, idx, sh.BodyLine)
		}
	}
}

func (c *checkCtx) checkEmitInBody(body, scriptName string, idx SignalIndex, baseLine int) {
	for _, match := range emitPrefixCheckRe.FindAllStringSubmatchIndex(body, -1) {
		ref := body[match[2]:match[3]]
		declScript, sigName := resolveSignalRef(ref, scriptName)
		key := declScript + "::" + sigName
		params, ok := idx[key]
		if !ok {
			continue
		}
		argsStart := match[1]
		depth := 1
		i := argsStart
		for i < len(body) && depth > 0 {
			switch body[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			i++
		}
		if depth != 0 {
			continue
		}
		argsStr := strings.TrimSpace(body[argsStart : i-1])
		argCount := 0
		if argsStr != "" {
			argCount = countTopLevelCommas(argsStr) + 1
		}
		if argCount != len(params) {
			c.add(emitArgCountMismatch(sigName, len(params), argCount, c.span(baseLine)))
		}
	}
}

func resolveSignalRef(ref, currentScript string) (string, string) {
	if before, after, ok := strings.Cut(ref, "::"); ok {
		return before, after
	}
	return currentScript, ref
}

func countTopLevelCommas(s string) int {
	depth := 0
	count := 0
	for _, ch := range s {
		switch ch {
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

// --- Task 6: Reactive/Watch/Empty rules ---

func (c *checkCtx) checkWatches(script ast.Script) {
	reactiveFields := map[string]bool{}
	for _, f := range script.Fields {
		if f.Modifier == ast.FieldReactive || f.Modifier == ast.FieldDerived {
			reactiveFields[f.Name] = true
		}
	}
	for _, w := range script.Watches {
		fieldName := strings.TrimPrefix(w.Field, "self.")
		if !reactiveFields[fieldName] {
			c.add(watchOnNonReactive(w.Field, c.span(w.Line)))
		}
	}
	for _, f := range script.Fields {
		if f.Modifier != ast.FieldDerived {
			continue
		}
		hasReactiveDep := false
		for _, rf := range script.Fields {
			if rf.Modifier == ast.FieldReactive || rf.Modifier == ast.FieldDerived {
				if strings.Contains(f.Default, "self."+rf.Name) {
					hasReactiveDep = true
					break
				}
			}
		}
		if !hasReactiveDep {
			c.add(derivedWithoutReactiveDep(f.Name, c.span(f.Line)))
		}
	}
}

func (c *checkCtx) checkEmpty(script ast.Script) {
	if len(script.Fields) == 0 && len(script.Handlers) == 0 && len(script.States) == 0 &&
		len(script.Signals) == 0 && len(script.Connects) == 0 && len(script.Watches) == 0 {
		c.add(emptyScript(script.Name, c.span(script.Line)))
	}
}
