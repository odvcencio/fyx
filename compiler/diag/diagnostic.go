// Package diag defines user-facing diagnostics emitted by compiler stages.
package diag

import "github.com/odvcencio/fyx/compiler/span"

// Severity is the display level for a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

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
}

// HasPrimary reports whether the diagnostic points at a concrete source span.
func (d Diagnostic) HasPrimary() bool {
	return !d.Primary.IsZero()
}
