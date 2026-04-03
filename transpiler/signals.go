package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// toPascalCase converts a snake_case or lowercase name to PascalCase.
// Examples: "died" -> "Died", "health_changed" -> "HealthChanged".
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

// signalMsgName returns the message struct name for a signal.
// Convention: {ScriptName}{SignalName}Msg, all PascalCase.
func signalMsgName(scriptName, signalName string) string {
	return toPascalCase(scriptName) + toPascalCase(signalName) + "Msg"
}

// TranspileSignalStructs generates Rust message struct declarations for a script's signals.
//
// Fyx:
//
//	signal died(position: Vector3)
//
// Becomes:
//
//	#[derive(Debug, Clone)]
//	pub struct EnemyDiedMsg {
//	    pub position: Vector3,
//	}
func TranspileSignalStructs(scriptName string, signals []ast.Signal) string {
	e := NewEmitter()
	EmitSignalStructs(e, scriptName, signals)
	return e.String()
}

// EmitSignalStructs writes signal message struct declarations into an emitter.
func EmitSignalStructs(e *RustEmitter, scriptName string, signals []ast.Signal) {
	for i, sig := range signals {
		if i > 0 {
			e.Blank()
		}
		name := signalMsgName(scriptName, sig.Name)
		e.LineWithSource("#[derive(Debug, Clone)]", sig.Line)
		e.LineWithSource(fmt.Sprintf("pub struct %s {", name), sig.Line)
		e.Indent()
		for _, p := range sig.Params {
			e.LineWithSource(fmt.Sprintf("pub %s: %s,", p.Name, p.TypeExpr), sig.Line)
		}
		e.Dedent()
		e.LineWithSource("}", sig.Line)
	}
}

// TranspileConnectSubscriptions generates subscribe_to calls for on_start from connect blocks.
//
// Fyx:
//
//	connect Enemy::died(pos) { ... }
//
// Becomes:
//
//	ctx.message_dispatcher.subscribe_to::<EnemyDiedMsg>(ctx.handle);
func TranspileConnectSubscriptions(connects []ast.Connect) string {
	if len(connects) == 0 {
		return ""
	}

	var lines []string
	for _, c := range connects {
		name := signalMsgName(c.ScriptName, c.SignalName)
		lines = append(lines, fmt.Sprintf("ctx.message_dispatcher.subscribe_to::<%s>(ctx.handle);", name))
	}
	return strings.Join(lines, "\n")
}

// TranspileConnectDispatch generates the on_message if-let chain for connect blocks.
//
// Fyx:
//
//	connect Enemy::died(pos) {
//	    self.score += 100;
//	}
//
// Becomes:
//
//	if let Some(msg) = message.downcast_ref::<EnemyDiedMsg>() {
//	    let pos = &msg.position;
//	    self.score += 100;
//	}
//
// Multiple connect blocks produce chained if-let blocks.
func TranspileConnectDispatch(connects []ast.Connect, signalIndex SignalIndex) string {
	if len(connects) == 0 {
		return ""
	}

	e := NewEmitter()
	for i, c := range connects {
		if i > 0 {
			e.Blank()
		}
		name := signalMsgName(c.ScriptName, c.SignalName)
		e.Linef("if let Some(msg) = message.downcast_ref::<%s>() {", name)
		e.Indent()

		for i, paramName := range c.Params {
			fieldName := paramName
			if params := signalParamsFor(signalIndex, c.ScriptName, c.SignalName); i < len(params) && params[i].Name != "" {
				fieldName = params[i].Name
			}
			e.Linef("let %s = &msg.%s;", paramName, fieldName)
		}

		body := strings.TrimSpace(c.Body)
		if body != "" {
			for _, line := range strings.Split(body, "\n") {
				e.Line(strings.TrimRight(line, " \t"))
			}
		}

		e.Dedent()
		e.Line("}")
	}
	return e.String()
}

// emitPrefixRe matches the beginning of an emit statement: `emit SIGNAL_NAME(`
// The actual argument list is extracted by balanced-paren scanning, not regex.
var emitPrefixRe = regexp.MustCompile(`emit\s+(\w+)\(`)

// RewriteEmitStatements rewrites `emit` statements in handler bodies to Rust message sends.
//
// Fyx:
//
//	emit died(self.position())
//	emit damaged(10.0, ctx.handle) to target
//
// Becomes:
//
//	ctx.message_sender.send_global(EnemyDiedMsg { position: self.position() });
//	ctx.message_sender.send_to_target(target, EnemyDamagedMsg { amount: 10.0, source: ctx.handle });
//
// The function uses the signal definitions to map positional arguments to named fields.
// It handles nested parentheses in arguments (e.g., self.position()).
func RewriteEmitStatements(body string, scriptName string, signals []ast.Signal) string {
	// Build a lookup map from signal name to its parameter list.
	sigMap := make(map[string][]ast.Param)
	for _, sig := range signals {
		sigMap[sig.Name] = sig.Params
	}

	// Process emit statements by finding the prefix and then scanning for balanced parens.
	result := body
	for {
		loc := emitPrefixRe.FindStringIndex(result)
		if loc == nil {
			break
		}

		prefix := result[:loc[0]]
		match := emitPrefixRe.FindStringSubmatch(result[loc[0]:])
		sigName := match[1]

		// Find the balanced closing paren starting after the opening paren.
		argsStart := loc[0] + len(match[0])
		closeIdx := findBalancedParen(result, argsStart)
		if closeIdx < 0 {
			break // malformed, stop processing
		}
		argsStr := result[argsStart:closeIdx]

		// Everything after the closing paren up to the semicolon.
		rest := result[closeIdx+1:]
		rest = strings.TrimLeft(rest, " \t")

		var replacement string
		msgName := signalMsgName(scriptName, sigName)
		fields := buildMsgFields(sigMap[sigName], argsStr)

		if strings.HasPrefix(rest, "to ") {
			// Targeted emit: `emit SIGNAL(ARGS) to TARGET;`
			afterTo := rest[3:]
			semiIdx := strings.Index(afterTo, ";")
			if semiIdx < 0 {
				break // malformed
			}
			target := strings.TrimSpace(afterTo[:semiIdx])
			replacement = fmt.Sprintf("ctx.message_sender.send_to_target(%s, %s { %s });", target, msgName, fields)
			result = prefix + replacement + afterTo[semiIdx+1:]
		} else {
			// Global emit: `emit SIGNAL(ARGS);`
			semiIdx := strings.Index(rest, ";")
			if semiIdx < 0 {
				break // malformed
			}
			replacement = fmt.Sprintf("ctx.message_sender.send_global(%s { %s });", msgName, fields)
			result = prefix + replacement + rest[semiIdx+1:]
		}
	}

	return result
}

// buildMsgFields pairs signal parameter names with argument expressions to produce
// Rust struct field initialization syntax like "position: self.position(), amount: 10.0".
func buildMsgFields(params []ast.Param, argsStr string) string {
	args := splitTopLevelCSV(argsStr)
	var fields []string
	for i, arg := range args {
		arg = strings.TrimSpace(arg)
		if i < len(params) {
			fields = append(fields, fmt.Sprintf("%s: %s", params[i].Name, arg))
		} else {
			// More args than params; emit positionally as a fallback.
			fields = append(fields, arg)
		}
	}
	return strings.Join(fields, ", ")
}
