package check

import (
	"fmt"

	"github.com/odvcencio/fyx/compiler/diag"
	"github.com/odvcencio/fyx/compiler/span"
)

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
