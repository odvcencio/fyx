package transpiler

import (
	"regexp"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// spawnRe matches `spawn EXPR at EXPR` patterns, capturing the resource expression
// and the position expression. The resource expression is non-greedy to stop at ` at `.
var spawnRe = regexp.MustCompile(`spawn\s+(\S+)\s+at\s+(.+)`)
var shorthandDtRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])dt\b`)
var ecsSpawnPrefixRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])ecs\s*\.\s*spawn\s*\(`)
var graphRotateYRe = regexp.MustCompile(`(ctx\.scene\.graph\[[^\]]+\])\.rotate_y\s*\(`)

// RewriteBody transforms Fyx shortcuts in handler bodies to valid Rust.
//
// Self-node shortcuts:
//   - self.position()  → ctx.scene.graph[ctx.handle].global_position()
//   - self.forward()   → ctx.scene.graph[ctx.handle].look_vector()
//   - self.parent()    → ctx.scene.graph[ctx.handle].parent()
//   - self.node.METHOD(...)  → ctx.scene.graph[ctx.handle].METHOD(...)
//   - self.node (standalone) → ctx.scene.graph[ctx.handle]
//
// Spawn syntax:
//   - spawn RESOURCE at POS → block that instantiates and positions the prefab
//   - ecs.spawn(A { ... }, B { ... }) → ctx.ecs.spawn((A { ... }, B { ... }))
//
// Regular self.field access (e.g., self.speed) is NOT rewritten.
func RewriteBody(body string, scriptName string, fields []ast.Field, kind ast.HandlerKind) string {
	handleExpr := "ctx.handle"
	if kind == ast.HandlerDeinit {
		handleExpr = "ctx.node_handle"
	}
	node := "ctx.scene.graph[" + handleExpr + "]"

	if kind == ast.HandlerUpdate && shorthandDtRe.MatchString(body) {
		body = "let dt = ctx.dt;\n" + body
	}

	// Order matters: replace self.node.METHOD before self.node (standalone).
	// Replace self.position(), self.forward(), self.parent() first (specific shortcuts).
	body = strings.ReplaceAll(body, "self.position()", node+".global_position()")
	body = strings.ReplaceAll(body, "self.forward()", node+".look_vector()")
	body = strings.ReplaceAll(body, "self.parent()", node+".parent()")

	for _, f := range fields {
		if f.Modifier != ast.FieldNode {
			continue
		}
		body = strings.ReplaceAll(body, "self."+f.Name+".", "ctx.scene.graph[self."+f.Name+"].")
	}

	// Replace self.node.METHOD(...) → ctx.scene.graph[ctx.handle].METHOD(...)
	// We need to handle "self.node." (with trailing dot) before standalone "self.node".
	body = strings.ReplaceAll(body, "self.node.", node+".")

	// Replace standalone self.node (word boundary — not followed by a dot or alphanumeric).
	// Use a regex to avoid replacing "self.node_something" or already-replaced "self.node.".
	standaloneNodeRe := regexp.MustCompile(`self\.node\b`)
	body = standaloneNodeRe.ReplaceAllString(body, node)
	body = graphRotateYRe.ReplaceAllString(body, `${1}.set_rotation_y(`)

	// Handle spawn syntax: spawn RESOURCE at POS
	body = spawnRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := spawnRe.FindStringSubmatch(match)
		resource := parts[1]
		pos := parts[2]
		// Strip trailing semicolon from pos if present — caller keeps statement structure.
		pos = strings.TrimRight(pos, ";")
		return "{ let _inst = ctx.resource_manager.request::<Model>(" + resource + ".clone()).instantiate(&mut ctx.scene.graph); " +
			"ctx.scene.graph[_inst].local_transform_mut().set_position(" + pos + "); _inst }"
	})

	return rewriteEcsSpawnCalls(body, "ctx.ecs")
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
