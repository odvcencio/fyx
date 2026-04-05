package diag

import (
	"strings"
	"testing"
)

func TestDiagnosticFormat_Verbose(t *testing.T) {
	d := Diagnostic{
		Code:     "F0002",
		Severity: SeverityError,
		Message:  "field `speed` has no type",
		Guide: GuideMessage{
			Summary: "`speed` needs a type.",
			Explain: "Every field must declare what kind of value it holds.",
			Suggest: "Try:\n\n    inspect speed: f32 = 0.0\n\n\"f32\" means a decimal number.",
		},
	}

	verbose := d.Format(true)
	if verbose == "" {
		t.Fatal("verbose format returned empty")
	}
	if !strings.Contains(verbose, "needs a type") {
		t.Errorf("verbose should contain guide summary, got:\n%s", verbose)
	}

	terse := d.Format(false)
	if !strings.Contains(terse, "F0002") {
		t.Errorf("terse should contain error code, got:\n%s", terse)
	}
	if strings.Contains(terse, "needs a type") {
		t.Errorf("terse should NOT contain guide text, got:\n%s", terse)
	}
}
