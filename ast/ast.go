package ast

// File is the root AST node — one per .fyx file.
type File struct {
	Imports      []Import
	Scripts      []Script
	Components   []Component
	Systems      []System
	ArbiterDecls []ArbiterDecl
	RustItems    []RustItem // passthrough Rust code
}

// Import represents a top-level module import.
type Import struct {
	Path string
	Line int
}

// Script represents a script block declaration.
type Script struct {
	Line     int
	Name     string
	Fields   []Field
	Handlers []Handler
	States   []State
	Signals  []Signal
	Connects []Connect
	Watches  []Watch
}

// Field represents a typed field with an optional modifier and default value.
type Field struct {
	Modifier FieldModifier // Inspect, Node, Nodes, Resource, Reactive, Derived, Bare
	Line     int
	Name     string
	TypeExpr string // raw type expression as string
	Default  string // raw default expression (empty if none)
}

// FieldModifier indicates how a field is exposed or bound.
type FieldModifier int

const (
	FieldBare FieldModifier = iota
	FieldInspect
	FieldNode
	FieldNodes
	FieldResource
	FieldTimer
	FieldReactive
	FieldDerived
)

// Handler represents a lifecycle or input/message handler within a script.
type Handler struct {
	Kind     HandlerKind // Init, Start, Update, Deinit, Event, Key, Mouse, Message
	Line     int
	BodyLine int
	Params   []Param
	Body     string // raw Rust body
}

// HandlerKind identifies the type of handler.
type HandlerKind int

const (
	HandlerInit HandlerKind = iota
	HandlerStart
	HandlerUpdate
	HandlerDeinit
	HandlerEvent
	HandlerKey
	HandlerMouse
	HandlerMessage
)

// State represents a state-machine block within a script.
type State struct {
	Line     int
	Name     string
	Handlers []StateHandler
}

// StateHandler represents an enter/update/exit block within a state.
type StateHandler struct {
	Kind     StateHandlerKind
	Line     int
	BodyLine int
	Body     string
}

// StateHandlerKind identifies the type of state-local handler.
type StateHandlerKind int

const (
	StateHandlerEnter StateHandlerKind = iota
	StateHandlerUpdate
	StateHandlerExit
)

// Param represents a named, optionally typed parameter.
type Param struct {
	Name     string
	TypeExpr string // empty for ctx (inferred)
}

// Signal represents a signal declaration within a script.
type Signal struct {
	Line   int
	Name   string
	Params []Param
}

// Connect represents a signal connection to another script's signal.
type Connect struct {
	Line       int
	BodyLine   int
	ScriptName string
	SignalName string
	Params     []string // binding names
	Body       string
}

// Watch represents a reactive watch on a field expression.
type Watch struct {
	Line     int
	BodyLine int
	Field    string // e.g. "self.is_critical"
	Body     string
}

// Component represents a standalone component declaration.
type Component struct {
	Line   int
	Name   string
	Fields []Field // always Bare modifier
}

// System represents a system declaration with queries and body code.
type System struct {
	Line     int
	BodyLine int
	Name     string
	Params   []Param // injected params (dt, etc.)
	Queries  []Query
	Body     string // non-query body code
}

// Query represents a query block within a system.
type Query struct {
	Line     int
	BodyLine int
	Params   []QueryParam
	Body     string
}

// QueryParam represents a single parameter in a query, possibly mutable.
type QueryParam struct {
	Name     string
	Mutable  bool
	TypeExpr string
}

// RustItem holds raw Rust source to be emitted unchanged.
type RustItem struct {
	Line   int
	Source string // raw Rust source, emitted unchanged
}

// ArbiterDeclKind identifies the governed declaration keyword.
type ArbiterDeclKind string

const (
	ArbiterDeclSource  ArbiterDeclKind = "source"
	ArbiterDeclWorker  ArbiterDeclKind = "worker"
	ArbiterDeclRule    ArbiterDeclKind = "rule"
	ArbiterDeclArbiter ArbiterDeclKind = "arbiter"
)

// ArbiterDecl preserves Arbiter-oriented declarations authored in .fyx files.
type ArbiterDecl struct {
	Kind   ArbiterDeclKind
	Line   int
	Name   string
	Body   string
	Source string
}
