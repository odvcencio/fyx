package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyrox-lang/ast"
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
	if !strings.Contains(out, "find_by_name") {
		t.Errorf("missing find_by_name for node resolution: %s", out)
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
	if !strings.Contains(out, "find_by_name") {
		t.Errorf("missing node resolution: %s", out)
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
	if !strings.Contains(out, `find_by_name_from_root("Camera3D")`) {
		t.Errorf("missing node resolution: %s", out)
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
	findIdx := strings.Index(out, "find_by_name")
	userIdx := strings.Index(out, "user_code")
	if findIdx < 0 || userIdx < 0 {
		t.Fatalf("missing resolution or user code: %s", out)
	}
	if findIdx > userIdx {
		t.Errorf("node resolution should come before user body: %s", out)
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
