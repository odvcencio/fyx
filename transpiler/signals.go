package transpiler

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

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
func TranspileConnectDispatch(connects []ast.Connect, currentScript string, fields []ast.Field, states []ast.State, signalIndex SignalIndex) string {
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
			body = RewriteEmitStatementsWithOptions(body, currentScript, EmitRewriteOptions{
				SignalIndex:    signalIndex,
				HandleBindings: analyzeScriptHandleBindings(body, fields, ast.HandlerMessage),
			})
			body = RewriteBodyWithStates(body, currentScript, fields, states, ast.HandlerMessage)
		}
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

// emitPrefixRe matches the beginning of an emit statement:
//   - `emit SIGNAL_NAME(`
//   - `emit ScriptName::signal_name(`
//
// The actual argument list is extracted by balanced-paren scanning, not regex.
var emitPrefixRe = regexp.MustCompile(`emit\s+([A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)?)\(`)

type EmitRewriteOptions struct {
	SignalIndex    SignalIndex
	HandleBindings handleBindingAnalysis
}

// RewriteEmitStatements rewrites `emit` statements in handler bodies to Rust message sends.
//
// Fyx:
//
//	emit died(self.position())
//	emit Enemy::damaged(amount: 10.0, source: ctx.handle) to target
//
// Becomes:
//
//	ctx.message_sender.send_global(EnemyDiedMsg { position: self.position() });
//	ctx.message_sender.send_to_target(target, EnemyDamagedMsg { amount: 10.0, source: ctx.handle });
//
// The function uses the project signal index to resolve bare local emits and
// script-qualified emits, mapping positional arguments to named fields.
// It handles nested parentheses in arguments (e.g., self.position()).
func RewriteEmitStatements(body string, currentScript string, signalIndex SignalIndex) string {
	return RewriteEmitStatementsWithOptions(body, currentScript, EmitRewriteOptions{SignalIndex: signalIndex})
}

func RewriteEmitStatementsWithOptions(body string, currentScript string, opts EmitRewriteOptions) string {
	result := body
	searchFrom := 0
	loopCounter := 0
	for {
		loc := emitPrefixRe.FindStringSubmatchIndex(result[searchFrom:])
		if loc == nil {
			break
		}

		start := searchFrom + loc[0]
		end := searchFrom + loc[1]
		signalRef := result[searchFrom+loc[2] : searchFrom+loc[3]]
		declScript, sigName := resolveEmitSignalRef(signalRef, currentScript)
		params := signalParamsFor(opts.SignalIndex, declScript, sigName)
		if params == nil {
			searchFrom = end
			continue
		}

		// Find the balanced closing paren starting after the opening paren.
		argsStart := end
		closeIdx := findBalancedParen(result, argsStart)
		if closeIdx < 0 {
			searchFrom = end
			continue
		}
		argsStr := result[argsStart:closeIdx]

		// Everything after the closing paren up to the semicolon.
		rest := result[closeIdx+1:]
		rest = strings.TrimLeft(rest, " \t")

		var replacement string
		msgName := signalMsgName(declScript, sigName)
		fields := buildMsgFields(params, argsStr)

		if strings.HasPrefix(rest, "to ") {
			// Targeted emit: `emit SIGNAL(ARGS) to TARGET;`
			afterTo := rest[3:]
			semiIdx := strings.Index(afterTo, ";")
			if semiIdx < 0 {
				searchFrom = end
				continue
			}
			target := strings.TrimSpace(afterTo[:semiIdx])
			if iterExpr, ok := collectionIteratorExpr(target, opts.HandleBindings); ok {
				loopVar := fmt.Sprintf("__fyx_target_%d", loopCounter)
				loopCounter++
				replacement = strings.Join([]string{
					fmt.Sprintf("for %s in %s {", loopVar, iterExpr),
					fmt.Sprintf("    ctx.message_sender.send_to_target(%s, %s { %s });", loopVar, msgName, fields),
					"}",
				}, "\n")
			} else {
				replacement = fmt.Sprintf("ctx.message_sender.send_to_target(%s, %s { %s });", target, msgName, fields)
			}
			result = result[:start] + replacement + afterTo[semiIdx+1:]
			searchFrom = start + len(replacement)
		} else {
			// Global emit: `emit SIGNAL(ARGS);`
			semiIdx := strings.Index(rest, ";")
			if semiIdx < 0 {
				searchFrom = end
				continue
			}
			replacement = fmt.Sprintf("ctx.message_sender.send_global(%s { %s });", msgName, fields)
			result = result[:start] + replacement + rest[semiIdx+1:]
			searchFrom = start + len(replacement)
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
		if fieldName, value, ok := splitNamedSignalArg(arg); ok {
			fields = append(fields, fmt.Sprintf("%s: %s", fieldName, value))
			continue
		}
		if i < len(params) {
			fields = append(fields, fmt.Sprintf("%s: %s", params[i].Name, arg))
		} else {
			// More args than params; emit positionally as a fallback.
			fields = append(fields, arg)
		}
	}
	return strings.Join(fields, ", ")
}

func resolveEmitSignalRef(signalRef, currentScript string) (scriptName string, signalName string) {
	if before, after, ok := strings.Cut(signalRef, "::"); ok {
		return before, after
	}
	return currentScript, signalRef
}

func splitNamedSignalArg(arg string) (fieldName string, value string, ok bool) {
	idx := findTopLevelNamedArgColon(arg)
	if idx < 0 {
		return "", "", false
	}

	fieldName = strings.TrimSpace(arg[:idx])
	value = strings.TrimSpace(arg[idx+1:])
	if fieldName == "" || value == "" || !isValidRustIdent(fieldName) {
		return "", "", false
	}
	return fieldName, value, true
}

func findTopLevelNamedArgColon(s string) int {
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	var quote byte
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ':':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				prevIsColon := i > 0 && s[i-1] == ':'
				nextIsColon := i+1 < len(s) && s[i+1] == ':'
				if !prevIsColon && !nextIsColon {
					return i
				}
			}
		}
	}
	return -1
}

func isValidRustIdent(s string) bool {
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}
