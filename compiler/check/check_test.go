package check

import (
	"testing"

	"github.com/odvcencio/fyx/ast"
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
