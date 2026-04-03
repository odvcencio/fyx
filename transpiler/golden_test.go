package transpiler

import (
	"os"
	"testing"

	"github.com/odvcencio/fyrox-lang/ast"
	"github.com/odvcencio/fyrox-lang/grammar"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

func goldenLang(t *testing.T) *gotreesitter.Language {
	t.Helper()
	g := grammar.FyroxScriptGrammar()
	l, err := grammargen.GenerateLanguage(g)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return l
}

func TestGoldenFiles(t *testing.T) {
	cases := []string{"minimal", "signals", "reactive", "ecs", "arbiter"}
	lang := goldenLang(t)
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile("../testdata/" + name + ".fyx")
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			expected, err := os.ReadFile("../testdata/golden/" + name + ".rs")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			file, err := ast.BuildAST(lang, input)
			if err != nil {
				t.Fatalf("build AST: %v", err)
			}
			got := TranspileFile(*file)
			if got != string(expected) {
				t.Errorf("golden mismatch for %s\n--- GOT ---\n%s\n--- WANT ---\n%s", name, got, string(expected))
			}
		})
	}
}
