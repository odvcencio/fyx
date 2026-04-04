package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// ReactiveExtra describes a shadow field that must be added to the struct
// for dirty-tracking a reactive or derived field.
type ReactiveExtra struct {
	Name     string // e.g., "_health_prev"
	TypeExpr string // e.g., "f32"
}

var selfFieldRefRe = regexp.MustCompile(`self\.([A-Za-z_][A-Za-z0-9_]*)`)

// prevFieldName returns the shadow field name for a given field name.
// Example: "health" -> "_health_prev"
func prevFieldName(name string) string {
	return "_" + name + "_prev"
}

func changeFlagName(name string) string {
	return "_fyx_" + name + "_changed"
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
// for dirty-tracking reactive fields.
//
// A reactive field like `reactive health: f32 = 100.0` produces:
//
//	ReactiveExtra{Name: "_health_prev", TypeExpr: "f32"}
func ReactiveFieldDecls(fields []ast.Field) []ReactiveExtra {
	var extras []ReactiveExtra
	for _, f := range fields {
		if f.Modifier == ast.FieldReactive {
			extras = append(extras, ReactiveExtra{
				Name:     prevFieldName(f.Name),
				TypeExpr: f.TypeExpr,
			})
		}
	}
	return extras
}

// ReactiveDefaultInits returns default init lines for shadow fields.
// Each reactive field's shadow is initialized to the same default
// as the source field (or the Rust Default::default() if none is specified).
//
// Example output line: "_health_prev: 100.0,"
func ReactiveDefaultInits(fields []ast.Field) []string {
	var lines []string
	for _, f := range fields {
		if f.Modifier == ast.FieldReactive {
			val := f.Default
			if val == "" {
				val = "Default::default()"
			}
			lines = append(lines, fmt.Sprintf("%s: %s,", prevFieldName(f.Name), val))
		}
	}
	return lines
}

func fieldLookup(fields []ast.Field) map[string]ast.Field {
	lookup := make(map[string]ast.Field, len(fields))
	for _, f := range fields {
		lookup[f.Name] = f
	}
	return lookup
}

func derivedFieldDependencies(expr string) []string {
	matches := selfFieldRefRe.FindAllStringSubmatchIndex(expr, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var deps []string
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		start, end := match[2], match[3]
		if end < len(expr) && expr[end] == '(' {
			continue
		}
		name := expr[start:end]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		deps = append(deps, name)
	}
	return deps
}

func canUseDerivedDependencyGating(deps []string, fieldsByName map[string]ast.Field) bool {
	if len(deps) == 0 {
		return false
	}
	for _, dep := range deps {
		field, ok := fieldsByName[dep]
		if !ok {
			return false
		}
		if field.Modifier != ast.FieldReactive && field.Modifier != ast.FieldDerived {
			return false
		}
	}
	return true
}

func orderedDerivedFields(fields []ast.Field) []ast.Field {
	var derived []ast.Field
	derivedByName := make(map[string]ast.Field)
	order := make(map[string]int)

	for _, f := range fields {
		if f.Modifier != ast.FieldDerived {
			continue
		}
		order[f.Name] = len(derived)
		derived = append(derived, f)
		derivedByName[f.Name] = f
	}
	if len(derived) <= 1 {
		return derived
	}

	inDegree := make(map[string]int, len(derived))
	dependents := make(map[string][]string, len(derived))
	for _, f := range derived {
		inDegree[f.Name] = 0
	}
	for _, f := range derived {
		for _, dep := range derivedFieldDependencies(f.Default) {
			if dep == f.Name {
				continue
			}
			if _, ok := derivedByName[dep]; !ok {
				continue
			}
			inDegree[f.Name]++
			dependents[dep] = append(dependents[dep], f.Name)
		}
	}

	var ready []string
	for _, f := range derived {
		if inDegree[f.Name] == 0 {
			ready = append(ready, f.Name)
		}
	}

	var ordered []ast.Field
	seen := make(map[string]struct{}, len(derived))
	for len(ready) > 0 {
		nextIdx := 0
		for i := 1; i < len(ready); i++ {
			if order[ready[i]] < order[ready[nextIdx]] {
				nextIdx = i
			}
		}
		name := ready[nextIdx]
		ready = append(ready[:nextIdx], ready[nextIdx+1:]...)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		ordered = append(ordered, derivedByName[name])
		for _, dependent := range dependents[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
	}

	if len(ordered) == len(derived) {
		return ordered
	}

	for _, f := range derived {
		if _, ok := seen[f.Name]; ok {
			continue
		}
		ordered = append(ordered, f)
	}
	return ordered
}

// GenerateReactiveUpdateCode generates the code to prepend/append to on_update
// for reactive dirty-tracking. The output has four sections in order:
//
//  1. Reactive change flags — compute whether each reactive field changed.
//  2. Derived recomputation — recompute derived fields only when tracked dependencies changed.
//  3. Watch conditionals — each watch block fires when the watched value's change flag is true.
//  4. Prev updates — reactive _prev fields are updated only when the value changed.
//
// Example output for a derived field `is_critical = self.health < 20.0` and
// a watch on `self.is_critical`:
//
//	let _fyx_health_changed = self.health != self._health_prev;
//	let _fyx_is_critical_changed = if _fyx_health_changed {
//	    let _fyx_is_critical_prev = self.is_critical.clone();
//	    self.is_critical = self.health < 20.0;
//	    self.is_critical != _fyx_is_critical_prev
//	} else {
//	    false
//	};
//	if _fyx_is_critical_changed {
//	    do_thing();
//	}
//	if _fyx_health_changed {
//	    self._health_prev = self.health.clone();
//	}
func GenerateReactiveUpdateCode(fields []ast.Field, watches []ast.Watch) string {
	e := NewEmitter()
	needsBlank := false
	fieldsByName := fieldLookup(fields)

	// 1. Reactive change flags
	for _, f := range fields {
		if f.Modifier != ast.FieldReactive {
			continue
		}
		if needsBlank {
			e.Blank()
		}
		e.Linef("let %s = self.%s != self.%s;", changeFlagName(f.Name), f.Name, prevFieldName(f.Name))
		needsBlank = true
	}

	// 2. Derived recomputation
	for _, f := range orderedDerivedFields(fields) {
		if f.Default == "" {
			continue
		}
		if needsBlank {
			e.Blank()
		}
		deps := derivedFieldDependencies(f.Default)
		changeName := changeFlagName(f.Name)
		prevValueName := "_fyx_" + f.Name + "_prev"
		if canUseDerivedDependencyGating(deps, fieldsByName) {
			var depFlags []string
			for _, dep := range deps {
				depFlags = append(depFlags, changeFlagName(dep))
			}
			e.Linef("let %s = if %s {", changeName, strings.Join(depFlags, " || "))
		} else {
			e.Linef("let %s = {", changeName)
		}
		e.Indent()
		e.Linef("let %s = self.%s.clone();", prevValueName, f.Name)
		e.Linef("self.%s = %s;", f.Name, f.Default)
		e.Linef("self.%s != %s", f.Name, prevValueName)
		e.Dedent()
		if canUseDerivedDependencyGating(deps, fieldsByName) {
			e.Line("} else {")
			e.Indent()
			e.Line("false")
			e.Dedent()
		}
		e.Line("};")
		needsBlank = true
	}

	// 3. Watch conditionals
	for _, w := range watches {
		if needsBlank {
			e.Blank()
		}
		fieldName := stripSelfPrefix(w.Field)
		e.Linef("if %s {", changeFlagName(fieldName))
		e.Indent()
		body := strings.TrimSpace(w.Body)
		if body != "" {
			for _, line := range strings.Split(body, "\n") {
				e.Line(strings.TrimRight(line, " \t"))
			}
		}
		e.Dedent()
		e.Line("}")
		needsBlank = true
	}

	// 4. Prev updates for reactive fields
	for _, f := range fields {
		if f.Modifier == ast.FieldReactive {
			if needsBlank {
				e.Blank()
			}
			e.Linef("if %s {", changeFlagName(f.Name))
			e.Indent()
			e.Linef("self.%s = self.%s.clone();", prevFieldName(f.Name), f.Name)
			e.Dedent()
			e.Line("}")
			needsBlank = true
		}
	}

	return strings.TrimRight(e.String(), "\n")
}
