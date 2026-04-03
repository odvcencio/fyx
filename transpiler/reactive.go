package transpiler

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// ReactiveExtra describes a shadow field that must be added to the struct
// for dirty-tracking a reactive or derived field.
type ReactiveExtra struct {
	Name     string // e.g., "_health_prev"
	TypeExpr string // e.g., "f32"
}

// prevFieldName returns the shadow field name for a given field name.
// Example: "health" -> "_health_prev"
func prevFieldName(name string) string {
	return "_" + name + "_prev"
}

// stripSelfPrefix removes the "self." prefix from a field expression if present.
// Example: "self.is_critical" -> "is_critical"
func stripSelfPrefix(s string) string {
	if strings.HasPrefix(s, "self.") {
		return s[5:]
	}
	return s
}

// ReactiveFieldDecls returns extra shadow field declarations to add to the struct
// for dirty-tracking reactive and derived fields.
//
// A reactive field like `reactive health: f32 = 100.0` produces:
//
//	ReactiveExtra{Name: "_health_prev", TypeExpr: "f32"}
//
// Derived fields also get shadow fields so watches can detect changes.
func ReactiveFieldDecls(fields []ast.Field) []ReactiveExtra {
	var extras []ReactiveExtra
	for _, f := range fields {
		if f.Modifier == ast.FieldReactive || f.Modifier == ast.FieldDerived {
			extras = append(extras, ReactiveExtra{
				Name:     prevFieldName(f.Name),
				TypeExpr: f.TypeExpr,
			})
		}
	}
	return extras
}

// ReactiveDefaultInits returns default init lines for shadow fields.
// Each reactive or derived field's shadow is initialized to the same default
// as the source field (or the Rust Default::default() if none is specified).
//
// Example output line: "_health_prev: 100.0,"
func ReactiveDefaultInits(fields []ast.Field) []string {
	var lines []string
	for _, f := range fields {
		if f.Modifier == ast.FieldReactive || f.Modifier == ast.FieldDerived {
			val := f.Default
			if val == "" {
				val = "Default::default()"
			}
			lines = append(lines, fmt.Sprintf("%s: %s,", prevFieldName(f.Name), val))
		}
	}
	return lines
}

// GenerateReactiveUpdateCode generates the code to prepend/append to on_update
// for reactive dirty-tracking. The output has three sections in order:
//
//  1. Derived recomputation — each derived field is recomputed from its expression.
//  2. Watch conditionals — each watch block fires when the watched value differs from _prev.
//  3. Prev updates — all reactive/derived _prev fields are updated to current values.
//
// Example output for a derived field `is_critical = self.health < 20.0` and
// a watch on `self.is_critical`:
//
//	self.is_critical = self.health < 20.0;
//	if self.is_critical != self._is_critical_prev {
//	    do_thing();
//	    self._is_critical_prev = self.is_critical.clone();
//	}
//	self._health_prev = self.health.clone();
func GenerateReactiveUpdateCode(fields []ast.Field, watches []ast.Watch) string {
	e := NewEmitter()
	needsBlank := false

	// 1. Derived recomputation
	for _, f := range fields {
		if f.Modifier == ast.FieldDerived && f.Default != "" {
			if needsBlank {
				e.Blank()
			}
			e.Linef("self.%s = %s;", f.Name, f.Default)
			needsBlank = true
		}
	}

	// 2. Watch conditionals
	for _, w := range watches {
		if needsBlank {
			e.Blank()
		}
		fieldName := stripSelfPrefix(w.Field)
		prev := "self." + prevFieldName(fieldName)
		e.Linef("if %s != %s {", w.Field, prev)
		e.Indent()
		body := strings.TrimSpace(w.Body)
		if body != "" {
			for _, line := range strings.Split(body, "\n") {
				e.Line(strings.TrimRight(line, " \t"))
			}
		}
		e.Linef("%s = %s.clone();", prev, w.Field)
		e.Dedent()
		e.Line("}")
		needsBlank = true
	}

	// 3. Prev updates for reactive fields (derived _prev is updated in watch blocks above,
	// but reactive fields need their _prev updated unconditionally)
	for _, f := range fields {
		if f.Modifier == ast.FieldReactive {
			if needsBlank {
				e.Blank()
			}
			e.Linef("self.%s = self.%s.clone();", prevFieldName(f.Name), f.Name)
			needsBlank = true
		}
	}

	return strings.TrimRight(e.String(), "\n")
}
