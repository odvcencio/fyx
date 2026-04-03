package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
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
// FyroxScript:
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
	if len(signals) == 0 {
		return ""
	}

	e := NewEmitter()
	for i, sig := range signals {
		if i > 0 {
			e.Blank()
		}
		name := signalMsgName(scriptName, sig.Name)
		e.Line("#[derive(Debug, Clone)]")
		e.Linef("pub struct %s {", name)
		e.Indent()
		for _, p := range sig.Params {
			e.Linef("pub %s: %s,", p.Name, p.TypeExpr)
		}
		e.Dedent()
		e.Line("}")
	}
	return e.String()
}

// TranspileConnectSubscriptions generates subscribe_to calls for on_start from connect blocks.
//
// FyroxScript:
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
// FyroxScript:
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
func TranspileConnectDispatch(connects []ast.Connect) string {
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

		// Bind each parameter from the connect block to the corresponding message field.
		// We look up the signal's param names to generate the correct field access.
		// Since we only have the connect's binding names and not the signal's param names,
		// the binding names ARE the field references (positional matching would require
		// the signal definition). For now, bind by position using the connect's param names
		// as both the local variable name and the msg field name.
		for _, paramName := range c.Params {
			e.Linef("let %s = &msg.%s;", paramName, paramName)
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
// FyroxScript:
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

// findBalancedParen scans from position start in s, counting parenthesis depth,
// and returns the index of the matching closing paren. Returns -1 if not found.
func findBalancedParen(s string, start int) int {
	depth := 1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// buildMsgFields pairs signal parameter names with argument expressions to produce
// Rust struct field initialization syntax like "position: self.position(), amount: 10.0".
func buildMsgFields(params []ast.Param, argsStr string) string {
	args := splitArgs(argsStr)
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

// splitArgs splits a comma-separated argument string, respecting parenthesized sub-expressions.
// For example, "foo(a, b), c" yields ["foo(a, b)", "c"].
func splitArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var result []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	result = append(result, strings.TrimSpace(s[start:]))
	return result
}
