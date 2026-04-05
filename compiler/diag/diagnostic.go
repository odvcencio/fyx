// Package diag defines user-facing diagnostics emitted by compiler stages.
package diag

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyx/compiler/span"
)

// Severity is the display level for a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

// GuideMessage holds beginner-friendly explanations for a diagnostic.
type GuideMessage struct {
	Summary string // one-line plain-language restatement
	Explain string // why this matters / what went wrong
	Suggest string // concrete fix with example code
}

// Label associates a span with a short explanatory message.
type Label struct {
	Span    span.Span
	Message string
}

// Diagnostic is a stage-agnostic compiler message.
type Diagnostic struct {
	Code     string
	Severity Severity
	Message  string
	Primary  span.Span
	Labels   []Label
	Notes    []string
	Guide    GuideMessage
}

// HasPrimary reports whether the diagnostic points at a concrete source span.
func (d Diagnostic) HasPrimary() bool {
	return !d.Primary.IsZero()
}

// Format renders the diagnostic as a human-readable string.
// When verbose is true and a guide message is present, the output uses
// beginner-friendly "guide-voice" formatting. Otherwise it uses terse
// compiler-voice formatting.
func (d Diagnostic) Format(verbose bool) string {
	if verbose && d.Guide.Summary != "" {
		return d.formatVerbose()
	}
	return d.formatTerse()
}

func (d Diagnostic) formatVerbose() string {
	var b strings.Builder

	// Location prefix when a primary span is available.
	if d.HasPrimary() {
		fmt.Fprintf(&b, "%s:%d — ", d.Primary.File, d.Primary.Start.Line)
	}

	b.WriteString(d.Guide.Summary)
	b.WriteByte('\n')

	if d.Guide.Explain != "" {
		b.WriteByte('\n')
		b.WriteString(d.Guide.Explain)
		b.WriteByte('\n')
	}

	if d.Guide.Suggest != "" {
		b.WriteByte('\n')
		b.WriteString(d.Guide.Suggest)
		b.WriteByte('\n')
	}

	return b.String()
}

func (d Diagnostic) formatTerse() string {
	var b strings.Builder

	// severity[code]: message
	fmt.Fprintf(&b, "%s[%s]: %s", d.Severity, d.Code, d.Message)

	// --> file:line:col
	if d.HasPrimary() {
		fmt.Fprintf(&b, " --> %s:%d:%d",
			d.Primary.File, d.Primary.Start.Line, d.Primary.Start.Column)
	}

	return b.String()
}
