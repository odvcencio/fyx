package transpiler

import (
	"fmt"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

// RenderArbiterBundle renders preserved Arbiter-oriented declarations back into
// .arb source so the build can emit a sidecar bundle for downstream tooling.
func RenderArbiterBundle(decls []ast.ArbiterDecl) string {
	if len(decls) == 0 {
		return ""
	}
	var parts []string
	for _, decl := range decls {
		if src := strings.TrimSpace(decl.Source); src != "" {
			parts = append(parts, src)
			continue
		}
		body := strings.TrimSpace(decl.Body)
		if body == "" {
			parts = append(parts, fmt.Sprintf("%s %s {}", decl.Kind, decl.Name))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s {\n%s\n}", decl.Kind, decl.Name, body))
	}
	return strings.Join(parts, "\n\n")
}

// EmitArbiterBundle exposes preserved Arbiter source in generated Rust so the
// bundle remains visible to downstream integration code even before full runtime
// wiring exists.
func EmitArbiterBundle(e *RustEmitter, decls []ast.ArbiterDecl) {
	if len(decls) == 0 {
		return
	}
	line := decls[0].Line
	if line == 0 {
		line = 1
	}
	e.LineWithSource(fmt.Sprintf("pub const FYX_ARBITER_BUNDLE: &str = %s;", rustRawStringLiteral(RenderArbiterBundle(decls))), line)
}

func rustRawStringLiteral(s string) string {
	hashes := "#"
	for strings.Contains(s, `"`+hashes) {
		hashes += "#"
	}
	return "r" + hashes + `"` + s + `"` + hashes
}
