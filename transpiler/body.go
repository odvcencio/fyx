package transpiler

import (
	"regexp"
	"slices"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// spawnRe matches `spawn EXPR at EXPR` patterns, capturing the resource expression
// and the position expression. The resource expression is non-greedy to stop at ` at `.
var spawnRe = regexp.MustCompile(`spawn\s+(\S+)\s+at\s+(.+)`)
var shorthandDtRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])dt\b`)
var ecsSpawnPrefixRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])ecs\s*\.\s*spawn\s*\(`)
var graphRotateYRe = regexp.MustCompile(`(ctx\.scene\.graph\[[^\]]+\])\.rotate_y\s*\(`)
var handleAliasLetRe = regexp.MustCompile(`(?m)^\s*let\s+(?:mut\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+?);\s*$`)
var handleNodeRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])([A-Za-z_][A-Za-z0-9_]*)\.node\(\)`)
var handleScriptRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])([A-Za-z_][A-Za-z0-9_]*)\.(script_mut|script)::<`)

// RewriteBody transforms Fyx shortcuts in handler bodies to valid Rust.
//
// Self-node shortcuts:
//   - self.position()  → ctx.scene.graph[ctx.handle].global_position()
//   - self.forward()   → ctx.scene.graph[ctx.handle].look_vector()
//   - self.parent()    → ctx.scene.graph[ctx.handle].parent()
//   - self.parent().node()      → ctx.scene.graph[ctx.scene.graph[ctx.handle].parent()]
//   - self.parent().script::<T>() → ctx.scene.graph[ctx.scene.graph[ctx.handle].parent()].script::<T>()
//   - self.muzzle.node()     → ctx.scene.graph[self.muzzle]
//   - self.muzzle.position() → ctx.scene.graph[self.muzzle].global_position()
//   - self.muzzle.forward()  → ctx.scene.graph[self.muzzle].look_vector()
//   - self.muzzle.parent()   → ctx.scene.graph[self.muzzle].parent()
//   - self.node.METHOD(...)  → ctx.scene.graph[ctx.handle].METHOD(...)
//   - self.node (standalone) → ctx.scene.graph[ctx.handle]
//   - projectile.node()      → ctx.scene.graph[projectile]
//   - projectile.script::<T>() → ctx.scene.graph[projectile].script::<T>()
//
// Spawn syntax:
//   - spawn RESOURCE at POS → block that instantiates and positions the prefab
//   - `resource ...: Model` fields spawn from the loaded resource directly
//   - other expressions are treated as model paths and requested via the resource manager
//   - ecs.spawn(A { ... }, B { ... }) → ctx.ecs.spawn((A { ... }, B { ... }))
//
// Regular self.field access (e.g., self.speed) is NOT rewritten.
func RewriteBody(body string, scriptName string, fields []ast.Field, kind ast.HandlerKind) string {
	handleExpr := "ctx.handle"
	if kind == ast.HandlerDeinit {
		handleExpr = "ctx.node_handle"
	}
	handleReceivers := expandHandleReceivers(body, nodeFieldReceivers(fields), []string{"self.parent()", handleExpr}, true)
	node := "ctx.scene.graph[" + handleExpr + "]"

	if kind == ast.HandlerUpdate && shorthandDtRe.MatchString(body) {
		body = "let dt = ctx.dt;\n" + body
	}

	parentNode := "ctx.scene.graph[" + node + ".parent()]"
	body = strings.ReplaceAll(body, "self.parent().node()", parentNode)
	body = strings.ReplaceAll(body, "self.parent().script_mut::<", parentNode+".script_mut::<")
	body = strings.ReplaceAll(body, "self.parent().script::<", parentNode+".script::<")

	// Order matters: replace self.node.METHOD before self.node (standalone).
	// Replace self.position(), self.forward(), self.parent() first (specific shortcuts).
	body = strings.ReplaceAll(body, "self.position()", node+".global_position()")
	body = strings.ReplaceAll(body, "self.forward()", node+".look_vector()")
	body = strings.ReplaceAll(body, "self.parent()", node+".parent()")

	for _, f := range fields {
		if f.Modifier != ast.FieldNode {
			continue
		}
		fieldNode := "ctx.scene.graph[self." + f.Name + "]"
		body = strings.ReplaceAll(body, "self."+f.Name+".node()", fieldNode)
		body = strings.ReplaceAll(body, "self."+f.Name+".position()", fieldNode+".global_position()")
		body = strings.ReplaceAll(body, "self."+f.Name+".forward()", fieldNode+".look_vector()")
		body = strings.ReplaceAll(body, "self."+f.Name+".parent()", fieldNode+".parent()")
		body = strings.ReplaceAll(body, "self."+f.Name+".", "ctx.scene.graph[self."+f.Name+"].")
	}

	// Replace self.node.METHOD(...) → ctx.scene.graph[ctx.handle].METHOD(...)
	// We need to handle "self.node." (with trailing dot) before standalone "self.node".
	body = strings.ReplaceAll(body, "self.node.", node+".")

	// Replace standalone self.node (word boundary — not followed by a dot or alphanumeric).
	// Use a regex to avoid replacing "self.node_something" or already-replaced "self.node.".
	standaloneNodeRe := regexp.MustCompile(`self\.node\b`)
	body = standaloneNodeRe.ReplaceAllString(body, node)
	body = rewriteHandleReceivers(body, handleReceivers)
	body = rewriteHandleNodeSugar(body)
	body = rewriteHandleScriptSugar(body)
	body = graphRotateYRe.ReplaceAllString(body, `${1}.set_rotation_y(`)

	// Handle spawn syntax: spawn RESOURCE at POS
	body = spawnRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := spawnRe.FindStringSubmatch(match)
		resource := parts[1]
		pos := parts[2]
		trailingSemicolon := strings.HasSuffix(strings.TrimSpace(pos), ";")
		// Strip trailing semicolon from pos if present — caller keeps statement structure.
		pos = strings.TrimRight(pos, ";")
		resourceExpr := spawnResourceExpr(resource, fields)
		suffix := ""
		if trailingSemicolon {
			suffix = ";"
		}
		return "{ let _resource = " + resourceExpr + "; let _inst = _resource.instantiate(&mut ctx.scene.graph); " +
			"ctx.scene.graph[_inst].local_transform_mut().set_position(" + pos + "); _inst }" + suffix
	})

	return rewriteEcsSpawnCalls(body, "ctx.ecs")
}

func spawnResourceExpr(resource string, fields []ast.Field) string {
	for _, f := range fields {
		if f.Modifier != ast.FieldResource || strings.TrimSpace(f.TypeExpr) != "Model" {
			continue
		}
		if resource == "self."+f.Name {
			return "self." + f.Name + `.clone().expect("Fyx resource field '` + f.Name + `' was not loaded before spawn")`
		}
	}
	return "ctx.resource_manager.request::<Model>(" + resource + ".clone())"
}

func nodeFieldReceivers(fields []ast.Field) []string {
	var receivers []string
	for _, f := range fields {
		if f.Modifier == ast.FieldNode {
			receivers = append(receivers, "self."+f.Name)
		}
	}
	return receivers
}

func expandHandleReceivers(body string, initialReceivers, extraProducers []string, allowSpawnAlias bool) []string {
	receivers := make([]string, 0, len(initialReceivers))
	seenReceivers := make(map[string]struct{}, len(initialReceivers))
	producers := make(map[string]struct{}, len(initialReceivers)+len(extraProducers))

	addReceiver := func(receiver string) bool {
		receiver = strings.TrimSpace(receiver)
		if receiver == "" {
			return false
		}
		if _, ok := seenReceivers[receiver]; ok {
			return false
		}
		seenReceivers[receiver] = struct{}{}
		receivers = append(receivers, receiver)
		producers[receiver] = struct{}{}
		return true
	}
	for _, receiver := range initialReceivers {
		addReceiver(receiver)
	}
	for _, producer := range extraProducers {
		producer = strings.TrimSpace(producer)
		if producer != "" {
			producers[producer] = struct{}{}
		}
	}

	changed := true
	for changed {
		changed = false
		for _, match := range handleAliasLetRe.FindAllStringSubmatch(body, -1) {
			if len(match) != 3 {
				continue
			}
			alias := match[1]
			rhs := strings.TrimSpace(match[2])
			if !isHandleProducer(rhs, producers, allowSpawnAlias) {
				continue
			}
			if addReceiver(alias) {
				changed = true
			}
		}
	}

	slices.SortFunc(receivers, func(a, b string) int {
		if len(a) == len(b) {
			return strings.Compare(a, b)
		}
		return len(b) - len(a)
	})
	return receivers
}

func isHandleProducer(expr string, producers map[string]struct{}, allowSpawnAlias bool) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if allowSpawnAlias && strings.HasPrefix(expr, "spawn ") {
		return true
	}
	if _, ok := producers[expr]; ok {
		return true
	}
	if strings.HasSuffix(expr, ".parent()") {
		base := strings.TrimSpace(strings.TrimSuffix(expr, ".parent()"))
		_, ok := producers[base]
		return ok
	}
	return false
}

func rewriteHandleReceivers(body string, receivers []string) string {
	for _, receiver := range receivers {
		node := "ctx.scene.graph[" + receiver + "]"
		parentNode := "ctx.scene.graph[" + node + ".parent()]"
		body = strings.ReplaceAll(body, receiver+".parent().node()", parentNode)
		body = strings.ReplaceAll(body, receiver+".parent().script_mut::<", parentNode+".script_mut::<")
		body = strings.ReplaceAll(body, receiver+".parent().script::<", parentNode+".script::<")
		body = strings.ReplaceAll(body, receiver+".node()", node)
		body = strings.ReplaceAll(body, receiver+".position()", node+".global_position()")
		body = strings.ReplaceAll(body, receiver+".forward()", node+".look_vector()")
		body = strings.ReplaceAll(body, receiver+".parent()", node+".parent()")
		body = strings.ReplaceAll(body, receiver+".script_mut::<", node+".script_mut::<")
		body = strings.ReplaceAll(body, receiver+".script::<", node+".script::<")
		body = strings.ReplaceAll(body, receiver+".", node+".")
	}
	return body
}

func rewriteHandleNodeSugar(body string) string {
	return handleNodeRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := handleNodeRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		leading, ident := parts[1], parts[2]
		if ident == "self" || ident == "ctx" {
			return match
		}
		return leading + "ctx.scene.graph[" + ident + "]"
	})
}

func rewriteHandleScriptSugar(body string) string {
	return handleScriptRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := handleScriptRe.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		leading, ident, method := parts[1], parts[2], parts[3]
		if ident == "self" || ident == "ctx" {
			return match
		}
		return leading + "ctx.scene.graph[" + ident + "]." + method + "::<"
	})
}

func rewriteEcsSpawnCalls(body, receiver string) string {
	result := body
	for {
		loc := ecsSpawnPrefixRe.FindStringSubmatchIndex(result)
		if loc == nil {
			return result
		}

		prefix := result[:loc[0]]
		leading := ""
		if loc[2] >= 0 {
			leading = result[loc[2]:loc[3]]
		}
		argsStart := loc[1]
		closeIdx := findBalancedParen(result, argsStart)
		if closeIdx < 0 {
			return result
		}

		args := splitTopLevelCSV(result[argsStart:closeIdx])
		bundle := "()"
		switch len(args) {
		case 0:
			bundle = "()"
		case 1:
			bundle = "(" + args[0] + ",)"
		default:
			bundle = "(" + strings.Join(args, ", ") + ")"
		}

		replacement := leading + receiver + ".spawn(" + bundle + ")"
		result = prefix + replacement + result[closeIdx+1:]
	}
}
