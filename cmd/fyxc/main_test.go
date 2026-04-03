package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOutputTreePreservesModules(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(inputDir, "combat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(inputDir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(rel, contents string) {
		path := filepath.Join(inputDir, rel)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("combat/weapon.fyx", "script Weapon {}\n")
	write("ui/hud.fyx", "import combat.weapon\nscript Hud {}\n")

	result, err := compileProject(inputDir)
	if err != nil {
		t.Fatalf("compileProject: %v", err)
	}

	outDir := filepath.Join(root, "generated")
	if err := writeOutputTree(outDir, result.Files, true); err != nil {
		t.Fatalf("writeOutputTree: %v", err)
	}

	hudOut, err := os.ReadFile(filepath.Join(outDir, "ui", "hud.rs"))
	if err != nil {
		t.Fatalf("read hud output: %v", err)
	}
	if !strings.Contains(string(hudOut), "use super::super::combat::weapon::*;") {
		t.Fatalf("expected relative import in hud.rs, got:\n%s", string(hudOut))
	}

	rootMod, err := os.ReadFile(filepath.Join(outDir, "mod.rs"))
	if err != nil {
		t.Fatalf("read root mod.rs: %v", err)
	}
	if !strings.Contains(string(rootMod), "pub mod combat;") || !strings.Contains(string(rootMod), "pub mod ui;") {
		t.Fatalf("unexpected root mod.rs:\n%s", string(rootMod))
	}

	uiMod, err := os.ReadFile(filepath.Join(outDir, "ui", "mod.rs"))
	if err != nil {
		t.Fatalf("read ui mod.rs: %v", err)
	}
	if strings.TrimSpace(string(uiMod)) != "pub mod hud;" {
		t.Fatalf("unexpected ui mod.rs:\n%s", string(uiMod))
	}

	if _, err := os.Stat(filepath.Join(outDir, "ui", "hud.fyxmap.json")); err != nil {
		t.Fatalf("expected source map sidecar: %v", err)
	}
}

func TestValidateGeneratedFilesPassesFixtureCorpus(t *testing.T) {
	result, err := compileProject(filepath.Join("..", "..", "testdata"))
	if err != nil {
		t.Fatalf("compileProject: %v", err)
	}

	diagnostics, err := validateGeneratedFiles(result.Files)
	if err != nil {
		t.Fatalf("validateGeneratedFiles: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got: %+v", diagnostics)
	}
}

func TestValidateGeneratedFilesMapsBackToSource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.fyx"), []byte(`script Broken {
    on update(ctx) {
        missing_call();
    }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := compileProject(root)
	if err != nil {
		t.Fatalf("compileProject: %v", err)
	}

	diagnostics, err := validateGeneratedFiles(result.Files)
	if err != nil {
		t.Fatalf("validateGeneratedFiles: %v", err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("expected a mapped diagnostic")
	}
	if diagnostics[0].SourcePath != "broken.fyx" {
		t.Fatalf("unexpected source path: %+v", diagnostics[0])
	}
	if diagnostics[0].SourceLine != 3 {
		t.Fatalf("expected source line 3, got %+v", diagnostics[0])
	}
}

func TestWriteOutputTreeEmitsArbiterSidecar(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "src")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "brain.fyx"), []byte(`worker decide_directive {
    input ThreatOutcome
    output NpcDirective
}

arbiter npc_brain {
    poll every_frame
    use_worker decide_directive
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := compileProject(inputDir)
	if err != nil {
		t.Fatalf("compileProject: %v", err)
	}

	outDir := filepath.Join(root, "generated")
	if err := writeOutputTree(outDir, result.Files, true); err != nil {
		t.Fatalf("writeOutputTree: %v", err)
	}

	arbOut, err := os.ReadFile(filepath.Join(outDir, "brain.arb"))
	if err != nil {
		t.Fatalf("read arbiter output: %v", err)
	}
	if !strings.Contains(string(arbOut), "worker decide_directive") || !strings.Contains(string(arbOut), "arbiter npc_brain") {
		t.Fatalf("unexpected arbiter sidecar:\n%s", string(arbOut))
	}
}
