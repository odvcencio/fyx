package transpiler

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

var shorthandDtRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])dt\b`)
var ecsSpawnPrefixRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])ecs\s*\.\s*spawn\s*\(`)
var graphRotateYRe = regexp.MustCompile(`(ctx\.scene\.graph\[[^\]]+\])\.rotate_y\s*\(`)
var scriptSceneRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])scene\.`)
var scriptGraphRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])graph\.`)
var scriptResourcesRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])resources\.`)
var scriptMessagesRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])messages\.`)
var scriptDispatcherRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])dispatcher\.`)
var scriptEcsRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])ecs\.`)
var scriptSceneLookupRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])(scene\.(?:find|find_all)(?:::<[^(\n]+>)?\(\s*"(?:[^"\\]|\\.)*"\s*\))`)
var scriptGraphLookupRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])(graph\.(?:find|find_all)(?:::<[^(\n]+>)?\(\s*"(?:[^"\\]|\\.)*"\s*\))`)
var handleAliasLetRe = regexp.MustCompile(`(?m)^\s*let\s+(?:mut\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+?);\s*$`)
var handleForLoopRe = regexp.MustCompile(`(?m)^\s*for\s+(?:mut\s+)?([A-Za-z_][A-Za-z0-9_]*)\s+in\s+(.+?)\s*\{\s*$`)
var handleNodeRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])([A-Za-z_][A-Za-z0-9_]*)\.node\(\)`)
var handleScriptRe = regexp.MustCompile(`(^|[^[:alnum:]_\.])([A-Za-z_][A-Za-z0-9_]*)\.(script_mut|script)::<`)

type handleBindingAnalysis struct {
	Receivers   []string
	Collections []string
	Producers   map[string]struct{}
}

type nodeLookupCall struct {
	Base     string
	Method   string
	TypeExpr string
	Path     string
}

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
// Script-context shorthands:
//   - scene.find("UI/Root")     → fyx_find_node_path(&ctx.scene.graph, "UI/Root")
//   - scene.find_all("UI/*")    → fyx_find_nodes_path(&ctx.scene.graph, "UI/*")
//   - scene.find::<Sprite>("UI/Crosshair") → fyx_expect_node_type::<Sprite>(...)
//   - scene.find_all::<Text>("UI/Digits/*") → fyx_expect_nodes_type::<Text>(...)
//   - scene.physics.raycast(...) → ctx.scene.physics.raycast(...)
//   - graph.remove_node(handle)  → ctx.scene.graph.remove_node(handle)
//   - resources.request::<T>(...) → ctx.resource_manager.request::<T>(...)
//   - messages.send_global(...)  → ctx.message_sender.send_global(...)
//   - dispatcher.subscribe_to::<T>(...) → ctx.message_dispatcher.subscribe_to::<T>(...)
//
// Spawn syntax:
//   - spawn RESOURCE at POS → block that instantiates and positions the prefab
//   - `resource ...: Model` fields spawn from the loaded resource directly
//   - other expressions are treated as model paths and requested via the resource manager
//   - ecs.spawn(A { ... }, B { ... }) → ctx.ecs.spawn((A { ... }, B { ... }))
//
// Regular self.field access (e.g., self.speed) is NOT rewritten.
func RewriteBody(body string, scriptName string, fields []ast.Field, kind ast.HandlerKind) string {
	return RewriteBodyWithStates(body, scriptName, fields, nil, kind)
}

func RewriteBodyWithStates(body string, scriptName string, fields []ast.Field, states []ast.State, kind ast.HandlerKind) string {
	handleExpr := "ctx.handle"
	if kind == ast.HandlerDeinit {
		handleExpr = "ctx.node_handle"
	}
	bindings := analyzeScriptHandleBindings(body, fields, kind)
	modelResourceAliases := modelResourceAliases(body, fields)
	node := "ctx.scene.graph[" + handleExpr + "]"

	if kind == ast.HandlerUpdate && shorthandDtRe.MatchString(body) {
		body = "let dt = ctx.dt;\n" + body
	}

	body = rewriteCollectionIterators(body, bindings)
	body = rewriteCollectionNodeCalls(body, bindings)
	body = rewriteRelativeNodeLookups(body, bindings)

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
	body = RewriteStateTransitions(body, scriptName, states)
	body = RewriteTimerSugar(body, fields)
	body = rewriteHandleReceivers(body, bindings.Receivers)
	body = rewriteHandleNodeSugar(body)
	body = rewriteHandleScriptSugar(body)
	body = rewriteRelativeNodeLookups(body, bindings)
	body = graphRotateYRe.ReplaceAllString(body, `${1}.set_rotation_y(`)

	body = rewriteSceneSpawnExpressions(body, fields, modelResourceAliases)

	body = rewriteEcsSpawnCalls(body, "ctx.ecs")
	return rewriteScriptContextShorthands(body, kind)
}

func spawnResourceExpr(resource string, fields []ast.Field, aliases map[string]string) string {
	for _, f := range fields {
		if f.Modifier != ast.FieldResource || strings.TrimSpace(f.TypeExpr) != "Model" {
			continue
		}
		if resource == "self."+f.Name {
			return "self." + f.Name + `.clone().expect("Fyx resource field '` + f.Name + `' was not loaded before spawn")`
		}
	}
	if fieldName, ok := aliases[strings.TrimSpace(resource)]; ok {
		return resource + `.clone().expect("Fyx resource field '` + fieldName + `' was not loaded before spawn")`
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

func nodesFieldIndexReceivers(body string, fields []ast.Field) []string {
	var receivers []string
	for _, f := range fields {
		if f.Modifier != ast.FieldNodes {
			continue
		}
		receivers = append(receivers, indexedHandleReceivers(body, "self."+f.Name)...)
	}
	return receivers
}

func nodesFieldCollections(fields []ast.Field) []string {
	var collections []string
	for _, f := range fields {
		if f.Modifier == ast.FieldNodes {
			collections = append(collections, "self."+f.Name)
		}
	}
	return collections
}

func indexedHandleReceivers(body, baseExpr string) []string {
	seen := make(map[string]struct{})
	var receivers []string
	re := regexp.MustCompile(regexp.QuoteMeta(baseExpr) + `\s*\[[^\]\n]+\]`)
	for _, match := range re.FindAllString(body, -1) {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		receivers = append(receivers, match)
	}
	return receivers
}

func modelResourceAliases(body string, fields []ast.Field) map[string]string {
	fieldByExpr := make(map[string]string)
	for _, f := range fields {
		if f.Modifier == ast.FieldResource && strings.TrimSpace(f.TypeExpr) == "Model" {
			fieldByExpr["self."+f.Name] = f.Name
		}
	}

	aliases := make(map[string]string)
	changed := true
	for changed {
		changed = false
		for _, match := range handleAliasLetRe.FindAllStringSubmatch(body, -1) {
			if len(match) != 3 {
				continue
			}
			alias := match[1]
			rhs := strings.TrimSpace(match[2])
			fieldName, ok := fieldByExpr[rhs]
			if !ok || aliases[alias] == fieldName {
				continue
			}
			aliases[alias] = fieldName
			fieldByExpr[alias] = fieldName
			changed = true
		}
	}

	return aliases
}

func analyzeScriptHandleBindings(body string, fields []ast.Field, kind ast.HandlerKind) handleBindingAnalysis {
	handleExpr := "ctx.handle"
	if kind == ast.HandlerDeinit {
		handleExpr = "ctx.node_handle"
	}
	return analyzeHandleBindings(
		body,
		append(nodeFieldReceivers(fields), nodesFieldIndexReceivers(body, fields)...),
		[]string{"self.parent()", handleExpr},
		nodesFieldCollections(fields),
		true,
	)
}

func analyzeHandleBindings(body string, initialReceivers, extraProducers, initialCollections []string, allowSpawnAlias bool) handleBindingAnalysis {
	receivers := make([]string, 0, len(initialReceivers))
	seenReceivers := make(map[string]struct{}, len(initialReceivers))
	producers := make(map[string]struct{}, len(initialReceivers)+len(extraProducers))
	collections := make(map[string]struct{}, len(initialCollections))

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
	addCollection := func(collection string) bool {
		collection = strings.TrimSpace(collection)
		if collection == "" {
			return false
		}
		if _, ok := collections[collection]; ok {
			return false
		}
		collections[collection] = struct{}{}
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
	for _, collection := range initialCollections {
		addCollection(collection)
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
			if isHandleProducer(rhs, producers, allowSpawnAlias) {
				if addReceiver(alias) {
					changed = true
				}
				continue
			}
			if isHandleCollectionProducer(rhs, producers, collections, allowSpawnAlias) {
				if addCollection(alias) {
					changed = true
				}
			}
		}
		for _, match := range handleForLoopRe.FindAllStringSubmatch(body, -1) {
			if len(match) != 3 {
				continue
			}
			alias := match[1]
			iterExpr := strings.TrimSpace(match[2])
			if !isHandleCollectionIterator(iterExpr, producers, collections, allowSpawnAlias) && !isHandleChildrenIterator(iterExpr, producers, allowSpawnAlias) {
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
	collectionList := make([]string, 0, len(collections))
	for collection := range collections {
		collectionList = append(collectionList, collection)
	}
	slices.SortFunc(collectionList, func(a, b string) int {
		if len(a) == len(b) {
			return strings.Compare(a, b)
		}
		return len(b) - len(a)
	})

	return handleBindingAnalysis{
		Receivers:   receivers,
		Collections: collectionList,
		Producers:   producers,
	}
}

func isHandleCollectionProducer(expr string, producers, collections map[string]struct{}, allowSpawnAlias bool) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if call, ok := parseGlobalNodeLookupCall(expr); ok && call.Method == "find_all" {
		return true
	}
	if call, ok := parseRelativeLookupCall(expr); ok && call.Method == "find_all" {
		return isHandleProducer(call.Base, producers, allowSpawnAlias)
	}
	if _, ok := collections[expr]; ok {
		return true
	}
	if strings.HasSuffix(expr, ".clone()") {
		base := strings.TrimSpace(strings.TrimSuffix(expr, ".clone()"))
		return isHandleCollectionProducer(base, producers, collections, allowSpawnAlias)
	}
	return false
}

func isHandleCollectionIterator(expr string, producers, collections map[string]struct{}, allowSpawnAlias bool) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if isHandleCollectionProducer(expr, producers, collections, allowSpawnAlias) {
		return true
	}
	for _, suffix := range []string{".into_iter()", ".iter().copied()", ".iter().cloned()"} {
		if strings.HasSuffix(expr, suffix) {
			base := strings.TrimSpace(strings.TrimSuffix(expr, suffix))
			return isHandleCollectionProducer(base, producers, collections, allowSpawnAlias)
		}
	}
	return false
}

func isHandleChildrenIterator(expr string, producers map[string]struct{}, allowSpawnAlias bool) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	for _, suffix := range []string{".children()", ".children().iter().copied()", ".children().iter().cloned()"} {
		if strings.HasSuffix(expr, suffix) {
			base := strings.TrimSpace(strings.TrimSuffix(expr, suffix))
			return isHandleProducer(base, producers, allowSpawnAlias)
		}
	}
	return false
}

func isHandleProducer(expr string, producers map[string]struct{}, allowSpawnAlias bool) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if call, ok := parseGlobalNodeLookupCall(expr); ok && call.Method == "find" {
		return true
	}
	if call, ok := parseRelativeLookupCall(expr); ok && call.Method == "find" {
		return isHandleProducer(call.Base, producers, allowSpawnAlias)
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

func rewriteCollectionIterators(body string, bindings handleBindingAnalysis) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		match := handleForLoopRe.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		iterExpr := strings.TrimSpace(match[2])
		replacement, ok := collectionIteratorExpr(iterExpr, bindings)
		if !ok || replacement == iterExpr {
			continue
		}
		lines[i] = strings.Replace(line, iterExpr, replacement, 1)
	}
	return strings.Join(lines, "\n")
}

func rewriteCollectionNodeCalls(body string, bindings handleBindingAnalysis) string {
	lines := strings.Split(body, "\n")
	counter := 0
	for i, line := range lines {
		indent, target, method, args, ok := parseCollectionMethodStatement(line)
		if !ok {
			continue
		}
		iterExpr, ok := collectionIteratorExpr(target, bindings)
		if !ok {
			continue
		}
		loopVar := fmt.Sprintf("__fyx_item_%d", counter)
		counter++
		lines[i] = strings.Join([]string{
			fmt.Sprintf("%sfor %s in %s {", indent, loopVar, iterExpr),
			fmt.Sprintf("%s    ctx.scene.graph[%s].%s(%s);", indent, loopVar, method, args),
			fmt.Sprintf("%s}", indent),
		}, "\n")
	}
	return strings.Join(lines, "\n")
}

func rewriteRelativeNodeLookups(body string, bindings handleBindingAnalysis) string {
	candidates := make([]string, 0, len(bindings.Producers))
	for producer := range bindings.Producers {
		candidates = append(candidates, producer)
	}
	slices.SortFunc(candidates, func(a, b string) int {
		if len(a) == len(b) {
			return strings.Compare(a, b)
		}
		return len(b) - len(a)
	})

	for _, candidate := range candidates {
		lookupRe := regexp.MustCompile(regexp.QuoteMeta(candidate) + `\.(?:find|find_all)(?:::<[^(\n]+>)?\(\s*"(?:[^"\\]|\\.)*"\s*\)`)
		body = lookupRe.ReplaceAllStringFunc(body, func(match string) string {
			call, ok := parseRelativeLookupCall(match)
			if !ok {
				return match
			}
			return renderRelativeLookupCall(call)
		})
	}

	graphLookupRe := regexp.MustCompile(`ctx\.scene\.graph\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\.(?:find|find_all)(?:::<[^(\n]+>)?\(\s*"(?:[^"\\]|\\.)*"\s*\)`)
	body = graphLookupRe.ReplaceAllStringFunc(body, func(match string) string {
		call, ok := parseRelativeLookupCall(match)
		if !ok {
			return match
		}
		return renderRelativeLookupCall(call)
	})

	return body
}

func bodyHasLocalBinding(body, ident string) bool {
	bindingRe := regexp.MustCompile(`(?m)^\s*(?:let|for)\s+(?:mut\s+)?` + regexp.QuoteMeta(ident) + `\b`)
	return bindingRe.MatchString(body)
}

func parseGlobalNodeLookupCall(expr string) (nodeLookupCall, bool) {
	call, ok := parseNodeLookupCall(expr)
	if !ok {
		return nodeLookupCall{}, false
	}
	if call.Base != "scene" && call.Base != "graph" {
		return nodeLookupCall{}, false
	}
	return call, true
}

func rewriteGlobalNodeLookups(body string, allowScene, allowGraph bool) string {
	if allowScene {
		body = rewriteLookupPattern(body, scriptSceneLookupRe, renderGlobalLookupCall)
	}
	if allowGraph {
		body = rewriteLookupPattern(body, scriptGraphLookupRe, renderGlobalLookupCall)
	}
	return body
}

func rewriteScriptContextShorthands(body string, kind ast.HandlerKind) string {
	allowScene := !bodyHasLocalBinding(body, "scene")
	allowGraph := !bodyHasLocalBinding(body, "graph")
	body = rewriteGlobalNodeLookups(body, allowScene, allowGraph)
	if allowScene {
		body = scriptSceneRe.ReplaceAllString(body, `${1}ctx.scene.`)
	}
	if allowGraph {
		body = scriptGraphRe.ReplaceAllString(body, `${1}ctx.scene.graph.`)
	}
	if !bodyHasLocalBinding(body, "resources") {
		body = scriptResourcesRe.ReplaceAllString(body, `${1}ctx.resource_manager.`)
	}
	if !bodyHasLocalBinding(body, "messages") {
		body = scriptMessagesRe.ReplaceAllString(body, `${1}ctx.message_sender.`)
	}
	if kind != ast.HandlerMessage && kind != ast.HandlerDeinit && !bodyHasLocalBinding(body, "dispatcher") {
		body = scriptDispatcherRe.ReplaceAllString(body, `${1}ctx.message_dispatcher.`)
	}
	if !bodyHasLocalBinding(body, "ecs") {
		body = scriptEcsRe.ReplaceAllString(body, `${1}ctx.ecs.`)
	}
	return body
}

func parseCollectionMethodStatement(line string) (indent string, target string, method string, args string, ok bool) {
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	indent = line[:indentLen]
	trimmed := strings.TrimSpace(line)
	if !strings.HasSuffix(trimmed, ";") {
		return "", "", "", "", false
	}
	trimmed = strings.TrimSuffix(trimmed, ";")
	if !strings.HasSuffix(trimmed, ")") {
		return "", "", "", "", false
	}

	openIdx := findTopLevelCallOpenParen(trimmed)
	if openIdx < 0 {
		return "", "", "", "", false
	}
	args = trimmed[openIdx+1 : len(trimmed)-1]

	methodSep := findTopLevelDotBefore(trimmed[:openIdx])
	if methodSep < 0 {
		return "", "", "", "", false
	}
	target = strings.TrimSpace(trimmed[:methodSep])
	method = strings.TrimSpace(trimmed[methodSep+1 : openIdx])
	if target == "" || !isValidRustIdent(method) {
		return "", "", "", "", false
	}
	return indent, target, method, args, true
}

func parseNodeLookupCall(expr string) (nodeLookupCall, bool) {
	expr = strings.TrimSpace(expr)
	if !strings.HasSuffix(expr, ")") {
		return nodeLookupCall{}, false
	}

	openIdx := findTopLevelCallOpenParen(expr)
	if openIdx < 0 {
		return nodeLookupCall{}, false
	}
	methodSep := findTopLevelDotBefore(expr[:openIdx])
	if methodSep < 0 {
		return nodeLookupCall{}, false
	}
	base := strings.TrimSpace(expr[:methodSep])
	methodExpr := strings.TrimSpace(expr[methodSep+1 : openIdx])
	path := strings.TrimSpace(expr[openIdx+1 : len(expr)-1])
	if base == "" {
		return nodeLookupCall{}, false
	}
	method := methodExpr
	typeExpr := ""
	if idx := strings.Index(methodExpr, "::<"); idx >= 0 {
		if !strings.HasSuffix(methodExpr, ">") {
			return nodeLookupCall{}, false
		}
		method = strings.TrimSpace(methodExpr[:idx])
		typeExpr = strings.TrimSpace(methodExpr[idx+3 : len(methodExpr)-1])
	}
	if method != "find" && method != "find_all" {
		return nodeLookupCall{}, false
	}
	if len(path) < 2 || path[0] != '"' || path[len(path)-1] != '"' {
		return nodeLookupCall{}, false
	}
	return nodeLookupCall{
		Base:     base,
		Method:   method,
		TypeExpr: typeExpr,
		Path:     path,
	}, true
}

func parseRelativeLookupCall(expr string) (nodeLookupCall, bool) {
	return parseNodeLookupCall(expr)
}

func renderExpectedLookup(call nodeLookupCall, expr string) string {
	if strings.TrimSpace(call.TypeExpr) == "" {
		return expr
	}
	typeExpr := strings.TrimSpace(call.TypeExpr)
	switch call.Method {
	case "find":
		return `fyx_expect_node_type::<` + typeExpr + `>(&ctx.scene.graph, ` + expr + `, ` + call.Path + `, "` + typeExpr + `")`
	case "find_all":
		return `fyx_expect_nodes_type::<` + typeExpr + `>(&ctx.scene.graph, ` + expr + `, ` + call.Path + `, "` + typeExpr + `")`
	default:
		return expr
	}
}

func renderRelativeLookupCall(call nodeLookupCall) string {
	switch call.Method {
	case "find":
		return renderExpectedLookup(call, `fyx_find_relative_node_path(&ctx.scene.graph, `+call.Base+`, `+call.Path+`)`)
	case "find_all":
		return renderExpectedLookup(call, `fyx_find_relative_nodes_path(&ctx.scene.graph, `+call.Base+`, `+call.Path+`)`)
	default:
		return call.Base
	}
}

func renderGlobalLookupCall(call nodeLookupCall) string {
	switch call.Method {
	case "find":
		return renderExpectedLookup(call, `fyx_find_node_path(&ctx.scene.graph, `+call.Path+`)`)
	case "find_all":
		return renderExpectedLookup(call, `fyx_find_nodes_path(&ctx.scene.graph, `+call.Path+`)`)
	default:
		return call.Base
	}
}

func rewriteLookupPattern(body string, re *regexp.Regexp, render func(nodeLookupCall) string) string {
	matches := re.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		prefixStart, prefixEnd := match[2], match[3]
		exprStart, exprEnd := match[4], match[5]

		out.WriteString(body[last:start])
		if prefixStart >= 0 {
			out.WriteString(body[prefixStart:prefixEnd])
		}
		call, ok := parseNodeLookupCall(body[exprStart:exprEnd])
		if !ok {
			out.WriteString(body[exprStart:exprEnd])
		} else {
			out.WriteString(render(call))
		}
		last = end
	}
	out.WriteString(body[last:])
	return out.String()
}

func findTopLevelCallOpenParen(s string) int {
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	var quote byte
	escaped := false
	openIdx := -1

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
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				openIdx = i
			}
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
		}
	}

	if parenDepth != 0 || braceDepth != 0 || bracketDepth != 0 {
		return -1
	}
	return openIdx
}

func findTopLevelDotBefore(s string) int {
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	var quote byte
	escaped := false
	lastDot := -1

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
		case '.':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				lastDot = i
			}
		}
	}

	return lastDot
}

func collectionIteratorExpr(expr string, bindings handleBindingAnalysis) (string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", false
	}
	if call, ok := parseGlobalNodeLookupCall(expr); ok && call.Method == "find_all" {
		iterExpr := `fyx_find_nodes_path(&ctx.scene.graph, ` + call.Path + `)`
		return renderExpectedLookup(call, iterExpr) + `.into_iter()`, true
	}
	if call, ok := parseRelativeLookupCall(expr); ok && call.Method == "find_all" {
		if !isHandleProducer(call.Base, bindings.Producers, false) {
			return "", false
		}
		iterExpr := `fyx_find_relative_nodes_path(&ctx.scene.graph, ` + call.Base + `, ` + call.Path + `)`
		return renderExpectedLookup(call, iterExpr) + `.into_iter()`, true
	}
	for _, collection := range bindings.Collections {
		if expr == collection {
			return expr + ".iter().cloned()", true
		}
	}
	for _, suffix := range []string{".children()", ".children().iter().copied()", ".children().iter().cloned()"} {
		if strings.HasSuffix(expr, suffix) {
			base := strings.TrimSpace(strings.TrimSuffix(expr, suffix))
			if !isHandleProducer(base, bindings.Producers, false) {
				return "", false
			}
			return base + ".children().to_vec().into_iter()", true
		}
	}
	return "", false
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
		lifetime, lifetimeConsumed := consumeEcsSpawnLifetimeClause(result[closeIdx+1:])

		args := splitTopLevelCSV(result[argsStart:closeIdx])
		if lifetime != "" {
			args = append(args, "FyxEntityLifetime { remaining: "+lifetime+" }")
		}
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
		result = prefix + replacement + result[closeIdx+1+lifetimeConsumed:]
	}
}

type sceneSpawnSpec struct {
	Start    int
	End      int
	Resource string
	Position string
	Lifetime string
}

func rewriteSceneSpawnExpressions(body string, fields []ast.Field, aliases map[string]string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		spec, ok := parseSceneSpawnLine(line)
		if !ok {
			continue
		}
		resourceExpr := spawnResourceExpr(spec.Resource, fields, aliases)
		replacement := "{ let _resource = " + resourceExpr + "; let _inst = _resource.instantiate(&mut ctx.scene.graph); " +
			"ctx.scene.graph[_inst].local_transform_mut().set_position(" + spec.Position + ");"
		if spec.Lifetime != "" {
			replacement += " self." + sceneLifetimeFieldName + ".push(FyxSceneLifetime { handle: _inst, remaining: " + spec.Lifetime + " });"
		}
		replacement += " _inst }"
		lines[i] = line[:spec.Start] + replacement + line[spec.End:]
	}
	return strings.Join(lines, "\n")
}

func parseSceneSpawnLine(line string) (sceneSpawnSpec, bool) {
	start := strings.Index(line, "spawn ")
	if start < 0 {
		return sceneSpawnSpec{}, false
	}

	cursor := start + len("spawn ")
	for cursor < len(line) && (line[cursor] == ' ' || line[cursor] == '\t') {
		cursor++
	}
	resourceStart := cursor
	for cursor < len(line) && line[cursor] != ' ' && line[cursor] != '\t' {
		cursor++
	}
	if resourceStart == cursor {
		return sceneSpawnSpec{}, false
	}
	resource := strings.TrimSpace(line[resourceStart:cursor])

	cursor += skipInlineWhitespace(line[cursor:])
	if !strings.HasPrefix(line[cursor:], "at") {
		return sceneSpawnSpec{}, false
	}
	cursor += len("at")
	cursor += skipInlineWhitespace(line[cursor:])
	if cursor >= len(line) {
		return sceneSpawnSpec{}, false
	}

	end := findTopLevelExprEnd(line, cursor)
	if end < 0 {
		end = len(line)
	}
	tail := strings.TrimSpace(line[cursor:end])
	if tail == "" {
		return sceneSpawnSpec{}, false
	}

	position := tail
	lifetime := ""
	if lifetimeIdx := findTopLevelKeyword(tail, "lifetime"); lifetimeIdx >= 0 {
		position = strings.TrimSpace(tail[:lifetimeIdx])
		lifetime = strings.TrimSpace(tail[lifetimeIdx+len("lifetime"):])
	}
	if position == "" {
		return sceneSpawnSpec{}, false
	}

	return sceneSpawnSpec{
		Start:    start,
		End:      end,
		Resource: resource,
		Position: position,
		Lifetime: lifetime,
	}, true
}

func bodyHasSceneSpawnLifetime(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		spec, ok := parseSceneSpawnLine(line)
		if ok && spec.Lifetime != "" {
			return true
		}
	}
	return false
}

func bodyHasEcsSpawnLifetime(body string) bool {
	for {
		loc := ecsSpawnPrefixRe.FindStringSubmatchIndex(body)
		if loc == nil {
			return false
		}
		argsStart := loc[1]
		closeIdx := findBalancedParen(body, argsStart)
		if closeIdx < 0 {
			return false
		}
		if lifetime, _ := consumeEcsSpawnLifetimeClause(body[closeIdx+1:]); lifetime != "" {
			return true
		}
		body = body[closeIdx+1:]
	}
}

func consumeEcsSpawnLifetimeClause(rest string) (string, int) {
	consumed := skipInlineWhitespace(rest)
	if !strings.HasPrefix(rest[consumed:], "lifetime") {
		return "", 0
	}
	next := consumed + len("lifetime")
	if next < len(rest) && isRustIdentChar(rest[next]) {
		return "", 0
	}
	next += skipInlineWhitespace(rest[next:])
	if next >= len(rest) {
		return "", 0
	}
	endRel := findTopLevelExprEnd(rest, next)
	if endRel < 0 {
		endRel = len(rest)
	}
	expr := strings.TrimSpace(rest[next:endRel])
	if expr == "" {
		return "", 0
	}
	return expr, endRel
}

func skipInlineWhitespace(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func isRustIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func findTopLevelExprEnd(s string, start int) int {
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	var quote byte
	escaped := false

	for i := start; i < len(s); i++ {
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
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				return i
			}
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
		case ';', ',':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				return i
			}
		}
	}

	return -1
}

func findTopLevelKeyword(s, keyword string) int {
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	var quote byte
	escaped := false

	for i := 0; i+len(keyword) <= len(s); i++ {
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
			continue
		case '(':
			parenDepth++
			continue
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			continue
		case '{':
			braceDepth++
			continue
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
			continue
		}

		if parenDepth != 0 || braceDepth != 0 || bracketDepth != 0 {
			continue
		}
		if !strings.HasPrefix(s[i:], keyword) {
			continue
		}
		beforeOK := i == 0 || !isRustIdentChar(s[i-1])
		afterIdx := i + len(keyword)
		afterOK := afterIdx >= len(s) || !isRustIdentChar(s[afterIdx])
		if beforeOK && afterOK {
			return i
		}
	}

	return -1
}
