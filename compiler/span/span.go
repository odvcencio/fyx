// Package span provides shared source location primitives for the compiler.
package span

// FileID identifies a source file within a compilation session.
type FileID string

// Point is a 1-based line/column position in a source file.
type Point struct {
	Line   int
	Column int
}

// Span identifies a half-open byte range in a source file and optionally keeps
// normalized line/column endpoints for human-facing diagnostics.
type Span struct {
	File      FileID
	StartByte int
	EndByte   int
	Start     Point
	End       Point
}

// IsZero reports whether the span is unset.
func (s Span) IsZero() bool {
	return s.File == "" && s.StartByte == 0 && s.EndByte == 0 &&
		s.Start == (Point{}) && s.End == (Point{})
}
