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
