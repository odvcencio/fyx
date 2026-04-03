package transpiler

import (
	"fmt"
	"strings"
)

// RustEmitter builds formatted Rust source code with indentation tracking.
type RustEmitter struct {
	buf    strings.Builder
	indent int
}

// NewEmitter creates a new RustEmitter with zero indentation.
func NewEmitter() *RustEmitter {
	return &RustEmitter{}
}

// Line emits a single line at the current indentation level.
func (e *RustEmitter) Line(s string) {
	for i := 0; i < e.indent; i++ {
		e.buf.WriteString("    ")
	}
	e.buf.WriteString(s)
	e.buf.WriteByte('\n')
}

// Linef emits a formatted line at the current indentation level.
func (e *RustEmitter) Linef(format string, args ...any) {
	e.Line(fmt.Sprintf(format, args...))
}

// Blank emits an empty line (no indentation).
func (e *RustEmitter) Blank() {
	e.buf.WriteByte('\n')
}

// Indent increases the indentation level by one.
func (e *RustEmitter) Indent() {
	e.indent++
}

// Dedent decreases the indentation level by one. It does not go below zero.
func (e *RustEmitter) Dedent() {
	if e.indent > 0 {
		e.indent--
	}
}

// String returns the accumulated Rust source code.
func (e *RustEmitter) String() string {
	return e.buf.String()
}
