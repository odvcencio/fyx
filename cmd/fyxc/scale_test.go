package main

import "testing"

func TestScaleProject_20Files(t *testing.T) {
	result, err := compileProject("../../testdata/scale")
	if err != nil {
		t.Fatalf("scale project compilation failed: %v", err)
	}

	if len(result.Files) < 20 {
		t.Errorf("expected at least 20 files, got %d", len(result.Files))
	}

	if result.TotalScripts < 10 {
		t.Errorf("expected at least 10 scripts, got %d", result.TotalScripts)
	}

	// Verify all files transpile without error
	for _, file := range result.Files {
		if file.Output.Code == "" {
			t.Errorf("empty output for %s", file.SourcePath)
		}
	}

	// Verify signal index resolves cross-file references (no error diagnostics about missing signals)
	errorCount := 0
	for _, d := range result.Diagnostics {
		if d.Severity == "error" {
			errorCount++
			t.Errorf("unexpected error diagnostic: %s: %s", d.Code, d.Message)
		}
	}
}
