package transpiler

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// hasNodeOrResourceFields returns true if any field has a Node, Nodes, or Resource modifier.
func hasNodeOrResourceFields(fields []ast.Field) bool {
	for _, f := range fields {
		switch f.Modifier {
		case ast.FieldNode, ast.FieldNodes, ast.FieldResource:
			return true
		}
	}
	return false
}

// GenerateNodeResolution generates the on_start body lines for resolving node and resource fields.
// Each node field produces a find_by_name call; each nodes field produces a wildcard match;
// each resource field produces a resource_manager.request call.
func GenerateNodeResolution(fields []ast.Field) string {
	var lines []string
	for _, f := range fields {
		lines = append(lines, resolutionLinesForField(f)...)
	}
	return strings.Join(lines, "\n")
}

func resolutionLinesForField(f ast.Field) []string {
	switch f.Modifier {
	case ast.FieldNode:
		name := unquote(f.Default)
		return []string{
			fmt.Sprintf("self.%s = ctx.scene.graph.find_by_name_from_root(\"%s\")", f.Name, name),
			"    .map(|(h, _)| h)",
			"    .unwrap_or_default();",
		}
	case ast.FieldNodes:
		pattern := unquote(f.Default)
		return []string{
			fmt.Sprintf("self.%s = ctx.scene.graph.find_by_name_from_root(\"%s\")", f.Name, pattern),
			"    .map(|(h, _)| h)",
			"    .into_iter()",
			"    .collect();",
		}
	case ast.FieldResource:
		path := strings.TrimPrefix(unquote(f.Default), "res://")
		return []string{
			fmt.Sprintf("self.%s = Some(ctx.resource_manager.request::<%s>(\"%s\"));", f.Name, f.TypeExpr, path),
		}
	default:
		return nil
	}
}

// nodeFieldRustType returns the Rust type for a node/nodes/resource field.
func nodeFieldRustType(f ast.Field) string {
	switch f.Modifier {
	case ast.FieldNode:
		return "Handle<Node>"
	case ast.FieldNodes:
		return "Vec<Handle<Node>>"
	case ast.FieldResource:
		return fmt.Sprintf("Option<Resource<%s>>", f.TypeExpr)
	default:
		return f.TypeExpr
	}
}

// unquote strips surrounding double quotes from a string, if present.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
