package ast

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// BuildAST parses source using the provided language and walks the CST to
// produce an ast.File.
func BuildAST(lang *gotreesitter.Language, source []byte) (*File, error) {
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(source)
	if err != nil {
		return nil, err
	}
	root := tree.RootNode()
	file := &File{}

	for i := 0; i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		switch child.Type(lang) {
		case "import_statement":
			file.Imports = append(file.Imports, buildImport(child, source, lang))
		case "script_declaration":
			file.Scripts = append(file.Scripts, buildScript(child, source, lang))
		case "component_declaration":
			file.Components = append(file.Components, buildComponent(child, source, lang))
		case "system_declaration":
			file.Systems = append(file.Systems, buildSystem(child, source, lang))
		case "rust_item":
			file.RustItems = append(file.RustItems, RustItem{
				Line:   sourceLine(child),
				Source: nodeText(child, source),
			})
		}
	}
	return file, nil
}

func sourceLine(n *gotreesitter.Node) int {
	if n == nil {
		return 0
	}
	return int(n.StartPoint().Row) + 1
}

// nodeText extracts the source text covered by a node.
func nodeText(n *gotreesitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	return n.Text(source)
}

func buildImport(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Import {
	return Import{
		Path: strings.ReplaceAll(nodeText(n.ChildByFieldName("path", lang), source), "::", "."),
		Line: sourceLine(n),
	}
}

// buildScript walks a script_declaration CST node and produces a Script.
func buildScript(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Script {
	s := Script{
		Line: sourceLine(n),
		Name: nodeText(n.ChildByFieldName("name", lang), source),
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		switch child.Type(lang) {
		case "inspect_field":
			s.Fields = append(s.Fields, buildField(child, source, lang, FieldInspect))
		case "node_field":
			s.Fields = append(s.Fields, buildField(child, source, lang, FieldNode))
		case "nodes_field":
			s.Fields = append(s.Fields, buildField(child, source, lang, FieldNodes))
		case "resource_field":
			s.Fields = append(s.Fields, buildField(child, source, lang, FieldResource))
		case "reactive_field":
			s.Fields = append(s.Fields, buildField(child, source, lang, FieldReactive))
		case "derived_field":
			s.Fields = append(s.Fields, buildDerivedField(child, source, lang))
		case "bare_field":
			s.Fields = append(s.Fields, buildField(child, source, lang, FieldBare))
		case "lifecycle_handler":
			s.Handlers = append(s.Handlers, buildHandler(child, source, lang))
		case "signal_declaration":
			s.Signals = append(s.Signals, buildSignal(child, source, lang))
		case "connect_block":
			s.Connects = append(s.Connects, buildConnect(child, source, lang))
		case "watch_block":
			s.Watches = append(s.Watches, buildWatch(child, source, lang))
		}
	}
	return s
}

// buildField extracts a field with a known modifier from a CST field node.
func buildField(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language, mod FieldModifier) Field {
	f := Field{
		Modifier: mod,
		Line:     sourceLine(n),
		Name:     nodeText(n.ChildByFieldName("name", lang), source),
		TypeExpr: nodeText(n.ChildByFieldName("type", lang), source),
	}
	if def := n.ChildByFieldName("default", lang); def != nil {
		f.Default = strings.TrimSpace(nodeText(def, source))
	}
	return f
}

// buildDerivedField extracts a derived field. Derived fields use "expression"
// instead of "default" as the field name for their value.
func buildDerivedField(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Field {
	f := Field{
		Modifier: FieldDerived,
		Line:     sourceLine(n),
		Name:     nodeText(n.ChildByFieldName("name", lang), source),
		TypeExpr: nodeText(n.ChildByFieldName("type", lang), source),
	}
	if expr := n.ChildByFieldName("expression", lang); expr != nil {
		f.Default = strings.TrimSpace(nodeText(expr, source))
	}
	return f
}

// buildHandler walks a lifecycle_handler CST node.
func buildHandler(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Handler {
	h := Handler{
		Line: sourceLine(n),
	}

	// Resolve handler kind from the "kind" field node text.
	kindNode := n.ChildByFieldName("kind", lang)
	if kindNode != nil {
		switch nodeText(kindNode, source) {
		case "init":
			h.Kind = HandlerInit
		case "start":
			h.Kind = HandlerStart
		case "update":
			h.Kind = HandlerUpdate
		case "deinit":
			h.Kind = HandlerDeinit
		case "event":
			h.Kind = HandlerEvent
		case "message":
			h.Kind = HandlerMessage
		}
	}

	// Extract parameters. Parameters are nested inside handler_parameters
	// which contains handler_parameter nodes.
	h.Params = collectHandlerParams(n, source, lang)

	// Extract body text (content between braces).
	if bodyNode := n.ChildByFieldName("body", lang); bodyNode != nil {
		h.BodyLine = sourceLine(bodyNode) + 1
		h.Body = extractBodyContent(bodyNode, source)
	}

	return h
}

// collectHandlerParams collects handler_parameter nodes from a parent that
// contains a handler_parameters child.
func collectHandlerParams(parent *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []Param {
	var params []Param
	for i := 0; i < parent.NamedChildCount(); i++ {
		child := parent.NamedChild(i)
		switch child.Type(lang) {
		case "handler_parameters":
			params = append(params, collectHandlerParamsFromList(child, source, lang)...)
		case "handler_parameter":
			params = append(params, buildParam(child, source, lang))
		}
	}
	return params
}

// collectHandlerParamsFromList extracts params from a handler_parameters list node.
func collectHandlerParamsFromList(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []Param {
	var params []Param
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child.Type(lang) == "handler_parameter" {
			params = append(params, buildParam(child, source, lang))
		}
	}
	return params
}

// buildParam extracts a single handler_parameter node into a Param.
func buildParam(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Param {
	p := Param{
		Name: nodeText(n.ChildByFieldName("name", lang), source),
	}
	if typeNode := n.ChildByFieldName("type", lang); typeNode != nil {
		p.TypeExpr = nodeText(typeNode, source)
	}
	return p
}

// buildSignal walks a signal_declaration CST node.
func buildSignal(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Signal {
	sig := Signal{
		Line: sourceLine(n),
		Name: nodeText(n.ChildByFieldName("name", lang), source),
	}
	params := collectHandlerParams(n, source, lang)
	sig.Params = params
	return sig
}

// buildConnect walks a connect_block CST node.
func buildConnect(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Connect {
	c := Connect{
		Line: sourceLine(n),
	}

	// Extract script::signal path from the "signal" field.
	sigPath := n.ChildByFieldName("signal", lang)
	if sigPath != nil {
		c.ScriptName = nodeText(sigPath.ChildByFieldName("script", lang), source)
		c.SignalName = nodeText(sigPath.ChildByFieldName("name", lang), source)
	}

	// Extract binding parameter names.
	params := collectHandlerParams(n, source, lang)
	for _, p := range params {
		c.Params = append(c.Params, p.Name)
	}

	// Extract body.
	if bodyNode := n.ChildByFieldName("body", lang); bodyNode != nil {
		c.BodyLine = sourceLine(bodyNode) + 1
		c.Body = extractBodyContent(bodyNode, source)
	}

	return c
}

// buildWatch walks a watch_block CST node.
func buildWatch(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Watch {
	w := Watch{
		Line: sourceLine(n),
	}

	// The target field contains a watch_target node: "self" "." identifier
	if target := n.ChildByFieldName("target", lang); target != nil {
		w.Field = nodeText(target, source)
	}

	if bodyNode := n.ChildByFieldName("body", lang); bodyNode != nil {
		w.BodyLine = sourceLine(bodyNode) + 1
		w.Body = extractBodyContent(bodyNode, source)
	}

	return w
}

// buildComponent walks a component_declaration CST node.
func buildComponent(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Component {
	comp := Component{
		Line: sourceLine(n),
		Name: nodeText(n.ChildByFieldName("name", lang), source),
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child.Type(lang) == "component_field" {
			comp.Fields = append(comp.Fields, Field{
				Modifier: FieldBare,
				Line:     sourceLine(child),
				Name:     nodeText(child.ChildByFieldName("name", lang), source),
				TypeExpr: nodeText(child.ChildByFieldName("type", lang), source),
			})
		}
	}
	return comp
}

// buildSystem walks a system_declaration CST node.
func buildSystem(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) System {
	sys := System{
		Line: sourceLine(n),
		Name: nodeText(n.ChildByFieldName("name", lang), source),
	}

	// Extract injected parameters (system_parameters -> handler_parameter nodes).
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		switch child.Type(lang) {
		case "system_parameters":
			for j := 0; j < child.NamedChildCount(); j++ {
				param := child.NamedChild(j)
				if param.Type(lang) == "handler_parameter" {
					sys.Params = append(sys.Params, buildParam(param, source, lang))
				}
			}
		case "handler_parameter":
			sys.Params = append(sys.Params, buildParam(child, source, lang))
		}
	}

	// Extract queries and body from the system_body node.
	if bodyNode := n.ChildByFieldName("body", lang); bodyNode != nil {
		sys.BodyLine = sourceLine(bodyNode) + 1
		sys.Queries = buildSystemQueries(bodyNode, source, lang)
		sys.Body = extractSystemBodyNonQuery(bodyNode, source, lang)
	}

	return sys
}

// buildSystemQueries extracts query_block nodes from a system_body node.
func buildSystemQueries(bodyNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []Query {
	var queries []Query
	for i := 0; i < bodyNode.NamedChildCount(); i++ {
		child := bodyNode.NamedChild(i)
		if child.Type(lang) == "query_block" {
			queries = append(queries, buildQuery(child, source, lang))
		}
	}
	return queries
}

// buildQuery walks a query_block CST node.
func buildQuery(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Query {
	q := Query{
		Line: sourceLine(n),
	}

	// Extract query parameters.
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		switch child.Type(lang) {
		case "query_parameters":
			for j := 0; j < child.NamedChildCount(); j++ {
				param := child.NamedChild(j)
				if param.Type(lang) == "query_parameter" {
					q.Params = append(q.Params, buildQueryParam(param, source, lang))
				}
			}
		case "query_parameter":
			q.Params = append(q.Params, buildQueryParam(child, source, lang))
		}
	}

	// Extract body.
	if bodyNode := n.ChildByFieldName("body", lang); bodyNode != nil {
		q.BodyLine = sourceLine(bodyNode) + 1
		q.Body = extractBodyContent(bodyNode, source)
	}

	return q
}

// buildQueryParam walks a query_parameter CST node.
func buildQueryParam(n *gotreesitter.Node, source []byte, lang *gotreesitter.Language) QueryParam {
	qp := QueryParam{
		Name: nodeText(n.ChildByFieldName("name", lang), source),
	}

	typeNode := n.ChildByFieldName("type", lang)
	if typeNode != nil {
		// query_type is: &mut Type | &Type | Type
		// We need to determine mutability and extract the base type.
		typeText := nodeText(typeNode, source)
		if strings.HasPrefix(typeText, "&mut ") || strings.HasPrefix(typeText, "&mut\t") {
			qp.Mutable = true
			qp.TypeExpr = strings.TrimSpace(typeText[4:])
		} else if strings.HasPrefix(typeText, "&") {
			qp.Mutable = false
			qp.TypeExpr = strings.TrimSpace(typeText[1:])
		} else {
			qp.TypeExpr = typeText
		}
	}

	return qp
}

// extractBodyContent extracts the text inside a handler_body or system_body
// node, stripping the outer braces and trimming whitespace.
func extractBodyContent(bodyNode *gotreesitter.Node, source []byte) string {
	text := nodeText(bodyNode, source)
	// Strip outer { and }
	if len(text) >= 2 && text[0] == '{' && text[len(text)-1] == '}' {
		text = text[1 : len(text)-1]
	}
	return strings.TrimSpace(text)
}

// extractSystemBodyNonQuery extracts the non-query body text from a system_body node.
// For now, this returns an empty string since the test doesn't check it.
func extractSystemBodyNonQuery(bodyNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	// Collect text ranges that are NOT query blocks.
	var parts []string
	for i := 0; i < bodyNode.NamedChildCount(); i++ {
		child := bodyNode.NamedChild(i)
		if child.Type(lang) != "query_block" {
			parts = append(parts, nodeText(child, source))
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}
