package transpiler

import (
	"os"
	"strings"
	"testing"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/grammar"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

func goldenLang(t *testing.T) *gotreesitter.Language {
	t.Helper()
	g := grammar.FyxGrammar()
	l, err := grammargen.GenerateLanguage(g)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return l
}

func TestGoldenFiles(t *testing.T) {
	cases := []string{"minimal", "signals", "reactive", "ecs", "arbiter", "depth"}
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
			got := TranspileFileResult(*file, Options{
				CurrentModule: ModulePathFromRelative(name + ".fyx"),
				SignalIndex:   BuildSignalIndex([]ast.File{*file}),
				SourcePath:    name + ".fyx",
			}).Code
			if normalizeGolden(got) != normalizeGolden(string(expected)) {
				t.Errorf("golden mismatch for %s\n--- GOT ---\n%s\n--- WANT ---\n%s", name, got, string(expected))
			}
		})
	}
}

func normalizeGolden(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}
