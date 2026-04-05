package check

import (
	"testing"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/compiler/diag"
)

func TestCheckFile_EmptyFile_NoDiagnostics(t *testing.T) {
	file := ast.File{}
	diags := CheckFile(file, CheckOptions{})
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty file, got %d", len(diags))
	}
}

func TestCheckFile_ValidScript_NoDiagnostics(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Default: "5.0", Line: 2},
			},
			Handlers: []ast.Handler{
				{Kind: ast.HandlerUpdate, Body: "self.speed += 1.0;", Line: 3, BodyLine: 3},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{})
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for valid script, got %d: %v", len(diags), diags)
	}
}

func TestRule_DuplicateFieldName(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Line: 2},
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "f32", Line: 3},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0007")
}

func TestRule_MissingFieldType(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "speed", TypeExpr: "", Line: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0002")
}

func TestRule_NodePathWithoutQuotes(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Fields: []ast.Field{
				{Modifier: ast.FieldNode, Name: "muzzle", TypeExpr: "Node", Default: "MuzzlePoint", Line: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0013")
}

func TestRule_DuplicateScript(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{
			{Name: "Player", Line: 1},
			{Name: "Player", Line: 5},
		},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0008")
}

// --- Task 4: Handler and State tests ---

func TestRule_GoToUnknownState(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Enemy",
			States: []ast.State{
				{Name: "idle", Line: 2, Handlers: []ast.StateHandler{
					{Kind: ast.StateHandlerUpdate, Body: "go flying;", Line: 3, BodyLine: 3},
				}},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "enemy.fyx"})
	assertHasCode(t, diags, "F0010")
}

func TestRule_DtOutsideUpdate(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Handlers: []ast.Handler{
				{Kind: ast.HandlerInit, Body: "let x = dt;", Line: 2, BodyLine: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "player.fyx"})
	assertHasCode(t, diags, "F0014")
}

func TestRule_DtInUpdate_NoDiagnostic(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Player",
			Handlers: []ast.Handler{
				{Kind: ast.HandlerUpdate, Body: "let x = dt;", Line: 2, BodyLine: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{})
	assertNoCode(t, diags, "F0014")
}

// --- Task 5: Signal tests ---

func TestRule_ConnectSignalNotFound(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "ScoreTracker",
			Connects: []ast.Connect{
				{ScriptName: "Enemy", SignalName: "died", Line: 2, BodyLine: 3, Body: "self.score += 1;"},
			},
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "score", TypeExpr: "i32", Line: 1},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "score.fyx", SignalIndex: SignalIndex{}})
	assertHasCode(t, diags, "F0005")
}

func TestRule_ConnectSignalFound_NoDiagnostic(t *testing.T) {
	idx := SignalIndex{
		"Enemy::died": []ast.Param{{Name: "position", TypeExpr: "Vector3"}},
	}
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "ScoreTracker",
			Connects: []ast.Connect{
				{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Line: 2, BodyLine: 3, Body: "self.score += 1;"},
			},
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "score", TypeExpr: "i32", Line: 1},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{SignalIndex: idx})
	assertNoCode(t, diags, "F0005")
}

func TestRule_EmitArgCountMismatch(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Weapon",
			Signals: []ast.Signal{
				{Name: "fired", Params: []ast.Param{
					{Name: "position", TypeExpr: "Vector3"},
					{Name: "direction", TypeExpr: "Vector3"},
				}, Line: 1},
			},
			Handlers: []ast.Handler{
				{Kind: ast.HandlerUpdate, Body: "emit fired(self.position());", Line: 3, BodyLine: 3},
			},
		}},
	}
	// Build local signal index
	idx := SignalIndex{"Weapon::fired": file.Scripts[0].Signals[0].Params}
	diags := CheckFile(file, CheckOptions{SignalIndex: idx})
	assertHasCode(t, diags, "F0006")
}

// --- Task 6: Reactive/Watch/Empty tests ---

func TestRule_WatchOnNonReactiveField(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "HUD",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "score", TypeExpr: "i32", Line: 1},
			},
			Watches: []ast.Watch{
				{Field: "self.score", Body: "println!(\"changed\");", Line: 3, BodyLine: 3},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "hud.fyx"})
	assertHasCode(t, diags, "F0011")
}

func TestRule_DerivedWithoutReactiveDep(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "HUD",
			Fields: []ast.Field{
				{Modifier: ast.FieldInspect, Name: "base", TypeExpr: "f32", Default: "10.0", Line: 1},
				{Modifier: ast.FieldDerived, Name: "doubled", TypeExpr: "f32", Default: "self.base * 2.0", Line: 2},
			},
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "hud.fyx"})
	assertHasCode(t, diags, "F0012")
}

func TestRule_EmptyScript(t *testing.T) {
	file := ast.File{
		Scripts: []ast.Script{{
			Name: "Empty",
			Line: 1,
		}},
	}
	diags := CheckFile(file, CheckOptions{FilePath: "empty.fyx"})
	assertHasCode(t, diags, "F0015")
}

// --- Helpers ---

func assertHasCode(t *testing.T, diags []diag.Diagnostic, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			return
		}
	}
	codes := make([]string, len(diags))
	for i, d := range diags {
		codes[i] = d.Code
	}
	t.Errorf("expected diagnostic code %s, got codes: %v", code, codes)
}

func assertNoCode(t *testing.T, diags []diag.Diagnostic, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			t.Errorf("did NOT expect diagnostic code %s but found: %s", code, d.Message)
		}
	}
}
