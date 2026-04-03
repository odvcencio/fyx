package transpiler

import (
	"path/filepath"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
)

// Options configures file transpilation in a project context.
type Options struct {
	CurrentModule []string
	SignalIndex   SignalIndex
	SourcePath    string
}

// GeneratedFile is the transpiled Rust output plus optional source mapping metadata.
type GeneratedFile struct {
	Code          string
	LineMap       []int
	ArbiterBundle string
}

// SourceMap maps generated line numbers back to a source .fyx file.
type SourceMap struct {
	SourcePath string `json:"source_path"`
	Lines      []int  `json:"lines"`
}

// Resolve returns the nearest mapped source line for a generated line number.
func (m SourceMap) Resolve(generatedLine int) int {
	if generatedLine <= 0 || generatedLine > len(m.Lines) {
		return 0
	}
	for i := generatedLine - 1; i >= 0; i-- {
		if m.Lines[i] != 0 {
			return m.Lines[i]
		}
	}
	return 0
}

// SignalIndex maps Script::signal identifiers to their declared parameters.
type SignalIndex map[string][]ast.Param

// BuildSignalIndex collects signal declarations across a set of parsed files.
func BuildSignalIndex(files []ast.File) SignalIndex {
	index := make(SignalIndex)
	for _, file := range files {
		for _, script := range file.Scripts {
			for _, sig := range script.Signals {
				key := signalIndexKey(script.Name, sig.Name)
				params := make([]ast.Param, len(sig.Params))
				copy(params, sig.Params)
				index[key] = params
			}
		}
	}
	return index
}

func signalIndexKey(scriptName, signalName string) string {
	return scriptName + "::" + signalName
}

func signalParamsFor(index SignalIndex, scriptName, signalName string) []ast.Param {
	if len(index) == 0 {
		return nil
	}
	return index[signalIndexKey(scriptName, signalName)]
}

func importSegments(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	path = strings.ReplaceAll(path, "::", ".")
	parts := strings.Split(path, ".")
	var out []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ModulePathFromRelative converts a relative .fyx path into Rust module segments.
func ModulePathFromRelative(rel string) []string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	if rel == "." || rel == "" {
		return nil
	}
	return strings.Split(rel, "/")
}

func relativeImportUsePath(currentModule, targetModule []string) string {
	if len(targetModule) == 0 {
		return ""
	}
	common := 0
	for common < len(currentModule) && common < len(targetModule) && currentModule[common] == targetModule[common] {
		common++
	}

	var parts []string
	for i := common; i < len(currentModule); i++ {
		parts = append(parts, "super")
	}
	parts = append(parts, targetModule[common:]...)
	if len(parts) == 0 {
		return "self"
	}
	return strings.Join(parts, "::")
}
