package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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

func TestParseArgsAcceptsWatchFlag(t *testing.T) {
	cmd, flagArgs, posArgs := parseArgs([]string{"build", "--watch", "--cargo-check", "testdata"})
	if cmd != "build" {
		t.Fatalf("unexpected command: %q", cmd)
	}
	if !slices.Contains(flagArgs, "--watch") {
		t.Fatalf("expected --watch in flag args, got %v", flagArgs)
	}
	if !slices.Equal(posArgs, []string{"testdata"}) {
		t.Fatalf("unexpected positional args: %v", posArgs)
	}
}

func TestScanProjectSnapshotDetectsEditsAddsAndDeletes(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.fyx")
	if err := os.WriteFile(first, []byte("script First {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := scanProjectSnapshot(root)
	if err != nil {
		t.Fatalf("scanProjectSnapshot before: %v", err)
	}

	if err := os.WriteFile(first, []byte("script First { value: i32 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	edited, err := scanProjectSnapshot(root)
	if err != nil {
		t.Fatalf("scanProjectSnapshot edited: %v", err)
	}
	if sameProjectSnapshot(before, edited) {
		t.Fatal("expected edited snapshot to differ")
	}

	second := filepath.Join(root, "second.fyx")
	if err := os.WriteFile(second, []byte("script Second {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := scanProjectSnapshot(root)
	if err != nil {
		t.Fatalf("scanProjectSnapshot added: %v", err)
	}
	if sameProjectSnapshot(edited, added) {
		t.Fatal("expected added snapshot to differ")
	}

	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}

	deleted, err := scanProjectSnapshot(root)
	if err != nil {
		t.Fatalf("scanProjectSnapshot deleted: %v", err)
	}
	if sameProjectSnapshot(added, deleted) {
		t.Fatal("expected deleted snapshot to differ")
	}
}
