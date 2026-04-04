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
		resolve := fmt.Sprintf("fyx_find_node_path(&ctx.scene.graph, \"%s\")", name)
		if !nodeFieldNeedsTypeValidation(f) {
			return []string{
				fmt.Sprintf("self.%s = %s;", f.Name, resolve),
			}
		}
		return []string{
			fmt.Sprintf("self.%s = fyx_expect_node_type::<%s>(&ctx.scene.graph, %s, \"%s\", \"%s\");", f.Name, f.TypeExpr, resolve, name, f.TypeExpr),
		}
	case ast.FieldNodes:
		pattern := unquote(f.Default)
		resolve := fmt.Sprintf("fyx_find_nodes_path(&ctx.scene.graph, \"%s\")", pattern)
		if !nodeFieldNeedsTypeValidation(f) {
			return []string{
				fmt.Sprintf("self.%s = %s;", f.Name, resolve),
			}
		}
		return []string{
			fmt.Sprintf("self.%s = fyx_expect_nodes_type::<%s>(&ctx.scene.graph, %s, \"%s\", \"%s\");", f.Name, f.TypeExpr, resolve, pattern, f.TypeExpr),
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

func nodeFieldNeedsTypeValidation(f ast.Field) bool {
	if f.Modifier != ast.FieldNode && f.Modifier != ast.FieldNodes {
		return false
	}
	typ := strings.TrimSpace(f.TypeExpr)
	return typ != "" && typ != "Node"
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
