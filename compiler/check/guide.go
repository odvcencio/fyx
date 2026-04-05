package check

import (
	"fmt"

	"github.com/odvcencio/fyx/compiler/diag"
	"github.com/odvcencio/fyx/compiler/span"
)

func missingFieldType(fieldName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0002",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("field `%s` has no type", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("`%s` needs a type.", fieldName),
			Explain: "Every field must declare what kind of value it holds.",
			Suggest: fmt.Sprintf("Try:\n\n    inspect %s: f32 = 0.0\n\n\"f32\" means a decimal number. Other common types:\n  i32    — whole number\n  bool   — true or false\n  String — text\n  Vector3 — 3D position (x, y, z)", fieldName),
		},
	}
}

func duplicateField(fieldName, scriptName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0007",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("duplicate field `%s` in script `%s`", fieldName, scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("There's already a field called `%s` in this script.", fieldName),
			Explain: "Each field name can only appear once per script.",
			Suggest: "Rename one of them or remove the duplicate.",
		},
	}
}

func duplicateScript(scriptName string, s span.Span, prevLine int) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0008",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("duplicate script `%s`", scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("Another script is already named `%s` (line %d).", scriptName, prevLine),
			Explain: "Script names must be unique within a file.",
			Suggest: "Rename one of them.",
		},
	}
}

func nodePathWithoutQuotes(fieldName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0013",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("node path for `%s` must be quoted", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "Node paths need quotes around them.",
			Explain: "The path tells Fyx where to find this node in your scene tree.",
			Suggest: fmt.Sprintf("Try:\n\n    node %s: Node = \"MuzzlePoint\"\n\nThe path uses \"/\" to go deeper: \"Turret/Muzzle\"", fieldName),
		},
	}
}
