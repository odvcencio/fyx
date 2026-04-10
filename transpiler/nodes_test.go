package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
)

func TestTranspileNodeField(t *testing.T) {
	s := ast.Script{
		Name: "Door",
		Fields: []ast.Field{
			{Modifier: ast.FieldNode, Name: "mesh", TypeExpr: "Mesh", Default: `"DoorMesh"`},
			{Modifier: ast.FieldResource, Name: "sound", TypeExpr: "SoundBuffer", Default: `"res://audio/creak.wav"`},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "Handle<Node>") {
		t.Errorf("missing Handle<Node>: %s", out)
	}
	if !strings.Contains(out, "fn on_start") {
		t.Errorf("missing on_start for node resolution: %s", out)
	}
	if !strings.Contains(out, `fyx_expect_node_type::<Mesh>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "DoorMesh"), "DoorMesh", "Mesh")`) {
		t.Errorf("missing typed path helper for node resolution: %s", out)
	}
}

func TestTranspileNodeWithUserOnStart(t *testing.T) {
	s := ast.Script{
		Name: "Player",
		Fields: []ast.Field{
			{Modifier: ast.FieldNode, Name: "cam", TypeExpr: "Camera", Default: `"MainCam"`},
		},
		Handlers: []ast.Handler{
			{Kind: ast.HandlerStart, Params: []ast.Param{{Name: "ctx"}}, Body: "println!(\"started\");"},
		},
	}
	out := TranspileScript(s)
	// Both node resolution AND user body should be in on_start
	if !strings.Contains(out, `fyx_expect_node_type::<Camera>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "MainCam"), "MainCam", "Camera")`) {
		t.Errorf("missing typed node resolution: %s", out)
	}
	if !strings.Contains(out, "println!") {
		t.Errorf("missing user body: %s", out)
	}
}

func TestTranspileNodesField(t *testing.T) {
	s := ast.Script{
		Name: "Machine",
		Fields: []ast.Field{
			{Modifier: ast.FieldNodes, Name: "gears", TypeExpr: "Mesh", Default: `"Gears/*"`},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "Vec<Handle<Node>>") {
		t.Errorf("missing Vec<Handle<Node>>: %s", out)
	}
	if !strings.Contains(out, "fn on_start") {
		t.Errorf("missing on_start for nodes resolution: %s", out)
	}
	if !strings.Contains(out, `fyx_expect_nodes_type::<Mesh>(&ctx.scene.graph, fyx_find_nodes_path(&ctx.scene.graph, "Gears/*"), "Gears/*", "Mesh")`) {
		t.Errorf("missing typed wildcard helper for nodes resolution: %s", out)
	}
}

func TestTranspileResourceField(t *testing.T) {
	s := ast.Script{
		Name: "AudioPlayer",
		Fields: []ast.Field{
			{Modifier: ast.FieldResource, Name: "footstep", TypeExpr: "SoundBuffer", Default: `"res://audio/footstep.wav"`},
		},
	}
	out := TranspileScript(s)
	if !strings.Contains(out, "Option<Resource<SoundBuffer>>") {
		t.Errorf("missing Option<Resource<SoundBuffer>>: %s", out)
	}
	if !strings.Contains(out, "fn on_start") {
		t.Errorf("missing on_start for resource loading: %s", out)
	}
	if !strings.Contains(out, `request::<SoundBuffer>("audio/footstep.wav")`) {
		t.Errorf("missing resource request or wrong path stripping: %s", out)
	}
}

func TestGenerateNodeResolution(t *testing.T) {
	fields := []ast.Field{
		{Modifier: ast.FieldNode, Name: "camera", TypeExpr: "Camera3D", Default: `"Camera3D"`},
		{Modifier: ast.FieldResource, Name: "sound", TypeExpr: "SoundBuffer", Default: `"res://audio/step.wav"`},
		{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32"}, // should be ignored
	}
	out := GenerateNodeResolution(fields)
	if !strings.Contains(out, `fyx_expect_node_type::<Camera3D>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "Camera3D"), "Camera3D", "Camera3D")`) {
		t.Errorf("missing typed node resolution: %s", out)
	}
	if !strings.Contains(out, `request::<SoundBuffer>("audio/step.wav")`) {
		t.Errorf("missing resource load: %s", out)
	}
	if strings.Contains(out, "speed") {
		t.Errorf("inspect field should not appear in resolution: %s", out)
	}
}

func TestNodeResolutionBeforeUserBody(t *testing.T) {
	s := ast.Script{
		Name: "Ordered",
		Fields: []ast.Field{
			{Modifier: ast.FieldNode, Name: "target", TypeExpr: "Node", Default: `"Target"`},
		},
		Handlers: []ast.Handler{
			{Kind: ast.HandlerStart, Params: []ast.Param{{Name: "ctx"}}, Body: "user_code();"},
		},
	}
	out := TranspileScript(s)
	findIdx := strings.Index(out, "fyx_find_node_path")
	userIdx := strings.Index(out, "user_code")
	if findIdx < 0 || userIdx < 0 {
		t.Fatalf("missing resolution or user code: %s", out)
	}
	if findIdx > userIdx {
		t.Errorf("node resolution should come before user body: %s", out)
	}
}

func TestGeneratedNodePathHelpers(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{
				Name: "TurretHud",
				Fields: []ast.Field{
					{Modifier: ast.FieldNode, Name: "heat_bar", TypeExpr: "ProgressBar", Default: `"UI/HeatBar"`},
					{Modifier: ast.FieldNodes, Name: "indicators", TypeExpr: "Node", Default: `"UI/*"`},
				},
			},
		},
	}

	out := TranspileFile(file)
	if !strings.Contains(out, "fn fyx_find_relative_node_path(graph: &Graph, root: Handle<Node>, path: &str) -> Handle<Node> {") {
		t.Fatalf("missing relative exact-path helper: %s", out)
	}
	if !strings.Contains(out, `".." => {`) {
		t.Fatalf("relative exact-path helper should support parent traversal: %s", out)
	}
	if !strings.Contains(out, "fn fyx_find_relative_nodes_path(graph: &Graph, root: Handle<Node>, pattern: &str) -> Vec<Handle<Node>> {") {
		t.Fatalf("missing relative wildcard helper: %s", out)
	}
	if !strings.Contains(out, `if pattern == "*" {`) {
		t.Fatalf("relative wildcard helper should support direct child collection lookup: %s", out)
	}
	if !strings.Contains(out, "fyx_find_relative_node_path(graph, root, parent_path)") {
		t.Fatalf("relative wildcard helper should resolve nested relative parent paths: %s", out)
	}
	if !strings.Contains(out, "fn fyx_find_node_path(graph: &Graph, path: &str) -> Handle<Node> {") {
		t.Fatalf("missing exact-path helper: %s", out)
	}
	if !strings.Contains(out, "current_node.children().iter().copied().find(|child| {") {
		t.Fatalf("path helper should walk direct child segments: %s", out)
	}
	if !strings.Contains(out, "node.name() == *segment") {
		t.Fatalf("path helper should compare child names against path segments: %s", out)
	}
	if !strings.Contains(out, `panic!("Fyx node path not found: {}", path)`) {
		t.Fatalf("path helper should fail loudly on missing paths: %s", out)
	}
	if !strings.Contains(out, "fn fyx_expect_node_type<T>(graph: &Graph, handle: Handle<Node>, path: &str, expected_type: &str) -> Handle<Node> {") {
		t.Fatalf("typed node helper missing: %s", out)
	}
	if !strings.Contains(out, "fn fyx_expect_nodes_type<T>(graph: &Graph, handles: Vec<Handle<Node>>, path: &str, expected_type: &str) -> Vec<Handle<Node>> {") {
		t.Fatalf("typed nodes helper missing: %s", out)
	}
	if !strings.Contains(out, `if let Some(parent_path) = pattern.strip_suffix("/*")`) {
		t.Fatalf("wildcard helper missing strip_suffix logic: %s", out)
	}
	if !strings.Contains(out, "graph.root()") {
		t.Fatalf("wildcard helper should support root children: %s", out)
	}
	if !strings.Contains(out, "node.children().to_vec()") {
		t.Fatalf("wildcard helper should collect child handles: %s", out)
	}
	if !strings.Contains(out, `fyx_expect_node_type::<ProgressBar>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "UI/HeatBar"), "UI/HeatBar", "ProgressBar")`) {
		t.Fatalf("typed node field should validate the resolved Fyrox node type: %s", out)
	}
}

func TestNodeFieldNotInDefaultImpl(t *testing.T) {
	s := ast.Script{
		Name: "WithDefaults",
		Fields: []ast.Field{
			{Modifier: ast.FieldNode, Name: "cam", TypeExpr: "Camera", Default: `"Cam"`},
			{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Default: "5.0"},
		},
	}
	out := TranspileScript(s)
	// The Default impl should contain speed but node defaults are resolved at runtime,
	// so the Default impl should still include the node field's default as a string
	// (it will be Handle::NONE by default via derive(Default))
	if !strings.Contains(out, "speed: 5.0") {
		t.Errorf("missing speed default: %s", out)
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`"hello"`, "hello"},
		{`hello`, "hello"},
		{`""`, ""},
		{`"a"`, "a"},
	}
	for _, tt := range tests {
		got := unquote(tt.in)
		if got != tt.want {
			t.Errorf("unquote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
