package ast

import "testing"

func TestScriptConstruction(t *testing.T) {
	s := Script{
		Name: "Player",
		Fields: []Field{
			{Modifier: FieldInspect, Name: "speed", TypeExpr: "f32", Default: "10.0"},
		},
		Handlers: []Handler{
			{Kind: HandlerUpdate, Params: []Param{{Name: "ctx"}}, Body: "self.speed += 1.0;"},
		},
	}
	if s.Name != "Player" {
		t.Errorf("expected Player, got %s", s.Name)
	}
	if len(s.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(s.Fields))
	}
}
