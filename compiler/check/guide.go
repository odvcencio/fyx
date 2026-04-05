package check

import (
	"fmt"
	"strings"

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

// --- Task 4: Handler and State guide messages ---

func goToUnknownState(targetState, scriptName string, knownStates []string, s span.Span) diag.Diagnostic {
	stateList := "none defined"
	if len(knownStates) > 0 {
		stateList = strings.Join(knownStates, ", ")
	}
	return diag.Diagnostic{
		Code:     "F0010",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("`go %s` but no state `%s` exists in `%s`", targetState, targetState, scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("`go %s` — but there's no state called `%s` in this script.", targetState, targetState),
			Explain: "The `go` keyword switches to another state. That state must be declared in the same script.",
			Suggest: fmt.Sprintf("Available states: %s\n\nTo add the missing state:\n\n    state %s {\n        on update { }\n    }", stateList, targetState),
		},
	}
}

func dtOutsideUpdate(handlerName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0014",
		Severity: diag.SeverityError,
		Message:  "`dt` used outside `on update`",
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "`dt` (delta time) is only available inside `on update`.",
			Explain: "Delta time is the seconds since the last frame. It only makes sense in code that runs every frame.",
			Suggest: "Move this code into an \"on update\" handler, or pass dt as a parameter from update.",
		},
	}
}

// --- Task 5: Signal guide messages ---

func signalNotFound(scriptName, signalName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0005",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("no signal `%s::%s` found", scriptName, signalName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("No signal `%s::%s` found.", scriptName, signalName),
			Explain: "A `connect` block listens for a signal from another script. That signal must be declared.",
			Suggest: fmt.Sprintf("Check that:\n  1. The script is named exactly \"%s\" (case matters)\n  2. It declares: signal %s(...)\n  3. The file containing \"%s\" is in the same project", scriptName, signalName, scriptName),
		},
	}
}

func emitArgCountMismatch(signalName string, expected, got int, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0006",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("`%s` expects %d args, got %d", signalName, expected, got),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: fmt.Sprintf("`%s` expects %d values but got %d.", signalName, expected, got),
			Explain: "When you emit a signal, you must provide all the values it was declared with.",
			Suggest: fmt.Sprintf("Check the signal declaration and make sure you're passing all %d arguments.", expected),
		},
	}
}

// --- Task 6: Reactive/Watch/Empty guide messages ---

func watchOnNonReactive(fieldExpr string, s span.Span) diag.Diagnostic {
	fieldName := strings.TrimPrefix(fieldExpr, "self.")
	return diag.Diagnostic{
		Code:     "F0011",
		Severity: diag.SeverityError,
		Message:  fmt.Sprintf("watch on non-reactive field `%s`", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "You can only `watch` reactive or derived fields.",
			Explain: fmt.Sprintf("`%s` is not declared as `reactive` or `derived`, so Fyx can't track when it changes.", fieldName),
			Suggest: fmt.Sprintf("To make it watchable, change the declaration to:\n\n    reactive %s: TYPE = DEFAULT", fieldName),
		},
	}
}

func derivedWithoutReactiveDep(fieldName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0012",
		Severity: diag.SeverityWarning,
		Message:  fmt.Sprintf("derived field `%s` doesn't reference any reactive fields", fieldName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "This derived field doesn't depend on any reactive fields.",
			Explain: "Derived fields recompute when their reactive dependencies change. Without any, this field never updates.",
			Suggest: fmt.Sprintf("Either:\n  - Reference a reactive field: derived %s: TYPE = self.REACTIVE_FIELD * 2\n  - Or change it to a bare field if it doesn't need auto-updating", fieldName),
		},
	}
}

func emptyScript(scriptName string, s span.Span) diag.Diagnostic {
	return diag.Diagnostic{
		Code:     "F0015",
		Severity: diag.SeverityWarning,
		Message:  fmt.Sprintf("script `%s` has no fields or handlers", scriptName),
		Primary:  s,
		Guide: diag.GuideMessage{
			Summary: "This script is empty — it won't do anything yet.",
			Explain: "Scripts need fields (data) or handlers (behavior) to be useful.",
			Suggest: fmt.Sprintf("Try adding something:\n\n    script %s {\n        inspect speed: f32 = 5.0\n\n        on update(ctx) {\n            // runs every frame\n        }\n    }", scriptName),
		},
	}
}
