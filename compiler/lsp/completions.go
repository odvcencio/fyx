package lsp

// CompletionItem represents a single auto-complete suggestion for the editor.
type CompletionItem struct {
	Label      string // displayed label
	Detail     string // short description shown beside the label
	InsertText string // VS Code snippet text ($1, $2, $0 are tab stops)
	Kind       int    // LSP CompletionItemKind (15 = Snippet, 14 = Keyword)
}

// CompletionItems returns the full catalog of Fyx keyword completions.
func CompletionItems() []CompletionItem {
	return []CompletionItem{
		// --- Top-level declarations ---
		{
			Label:      "script",
			Detail:     "Declare a new script block",
			InsertText: "script ${1:MyScript} {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "component",
			Detail:     "Declare a custom ECS component",
			InsertText: "component ${1:MyComponent} {\n\t${2:name}: ${3:Type}\n}",
			Kind:       15,
		},
		{
			Label:      "system",
			Detail:     "Declare an ECS system",
			InsertText: "system ${1:my_system}(${2:dt}) {\n\t$0\n}",
			Kind:       15,
		},

		// --- Field modifiers ---
		{
			Label:      "inspect",
			Detail:     "Editor-visible field",
			InsertText: "inspect ${1:name}: ${2:f32} = ${3:0.0}",
			Kind:       15,
		},
		{
			Label:      "node",
			Detail:     "Reference to a scene node",
			InsertText: "node ${1:name}: ${2:Node} = \"${3:NodePath}\"",
			Kind:       15,
		},
		{
			Label:      "nodes",
			Detail:     "Collection of scene nodes",
			InsertText: "nodes ${1:name}: ${2:Node} = \"${3:NodePath}\"",
			Kind:       15,
		},
		{
			Label:      "resource",
			Detail:     "External resource reference",
			InsertText: "resource ${1:name}: ${2:SoundBuffer} = \"${3:res://path}\"",
			Kind:       15,
		},
		{
			Label:      "timer",
			Detail:     "Countdown timer field",
			InsertText: "timer ${1:name} = ${2:1.0}",
			Kind:       15,
		},
		{
			Label:      "reactive",
			Detail:     "Automatically re-renders when changed",
			InsertText: "reactive ${1:name}: ${2:i32} = ${3:0}",
			Kind:       15,
		},
		{
			Label:      "derived",
			Detail:     "Computed value from reactive fields",
			InsertText: "derived ${1:name}: ${2:bool} = ${3:self.field > 0}",
			Kind:       15,
		},

		// --- Signals ---
		{
			Label:      "signal",
			Detail:     "Declare a signal that other scripts can connect to",
			InsertText: "signal ${1:name}(${2:param: Type})",
			Kind:       15,
		},
		{
			Label:      "emit",
			Detail:     "Fire a signal",
			InsertText: "emit ${1:signal_name}(${2:args});",
			Kind:       14,
		},
		{
			Label:      "connect",
			Detail:     "Listen to another script's signal",
			InsertText: "connect ${1:Script}::${2:signal}(${3:params}) {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "watch",
			Detail:     "React when a reactive field changes",
			InsertText: "watch self.${1:field} {\n\t$0\n}",
			Kind:       15,
		},

		// --- Lifecycle handlers ---
		{
			Label:      "on update",
			Detail:     "Runs every frame",
			InsertText: "on update(${1:ctx}) {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "on start",
			Detail:     "Runs once when the script starts",
			InsertText: "on start(${1:ctx}) {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "on init",
			Detail:     "Runs when the script is first created",
			InsertText: "on init(${1:ctx}) {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "on event",
			Detail:     "Handles a game event",
			InsertText: "on event(${1:evt}) {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "on deinit",
			Detail:     "Cleanup when the script is destroyed",
			InsertText: "on deinit(${1:ctx}) {\n\t$0\n}",
			Kind:       15,
		},

		// --- State machine ---
		{
			Label:      "state",
			Detail:     "Declare a named state in a state machine",
			InsertText: "state ${1:idle} {\n\ton enter {\n\t\t$0\n\t}\n\ton update {\n\t}\n}",
			Kind:       15,
		},

		// --- ECS ---
		{
			Label:      "query",
			Detail:     "Query entities with matching components",
			InsertText: "query(${1:name}: ${2:&Type}) {\n\t$0\n}",
			Kind:       15,
		},
		{
			Label:      "spawn",
			Detail:     "Spawn a new entity with components",
			InsertText: "spawn ${1:prefab} at ${2:position} lifetime ${3:1.0};",
			Kind:       15,
		},
	}
}
