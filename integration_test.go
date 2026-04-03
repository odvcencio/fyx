package fyx_test

import (
	"os"
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/grammar"
	"github.com/odvcencio/fyx/transpiler"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

func TestEndToEndFullExample(t *testing.T) {
	// Read the full example fixture
	source, err := os.ReadFile("testdata/full.fyx")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Generate language
	g := grammar.FyxGrammar()
	lang, err := grammargen.GenerateLanguage(g)
	if err != nil {
		t.Fatalf("generate grammar: %v", err)
	}

	// Parse
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sexpr := tree.RootNode().SExpr(lang)
	if strings.Contains(sexpr, "ERROR") {
		t.Fatalf("parse errors in full example:\n%s", sexpr)
	}

	// Build AST
	file, err := ast.BuildAST(lang, source)
	if err != nil {
		t.Fatalf("build AST: %v", err)
	}

	// Verify AST structure
	if len(file.Scripts) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(file.Scripts))
	}
	if len(file.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(file.Components))
	}
	if len(file.Systems) != 3 {
		t.Errorf("expected 3 systems, got %d", len(file.Systems))
	}

	// Transpile
	out := transpiler.TranspileFile(*file)

	// Verify key markers in output
	markers := []string{
		"struct Weapon",
		"impl ScriptTrait for Weapon",
		"struct WeaponHUD",
		"impl ScriptTrait for WeaponHUD",
		"struct Projectile",
		"struct Velocity",
		"fn system_move_projectiles",
		"fn system_expire_projectiles",
		"fn system_projectile_hits",
		"WeaponFiredMsg",
		"WeaponEmptiedMsg",
		"register_scripts",
		"run_ecs_systems",
		"subscribe_to",
		"Handle<Node>",
	}
	for _, m := range markers {
		if !strings.Contains(out, m) {
			t.Errorf("missing %q in output:\n%s", m, out)
		}
	}

	// Verify no duplicate sections
	weaponCount := strings.Count(out, "pub struct Weapon {")
	if weaponCount != 1 {
		t.Errorf("expected exactly 1 Weapon struct, found %d", weaponCount)
	}

	t.Logf("Full transpilation output (%d bytes):\n%s", len(out), out)
}
