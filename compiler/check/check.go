package check

import (
	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/compiler/diag"
	"github.com/odvcencio/fyx/compiler/span"
)

// SignalIndex maps "ScriptName::signalName" to declared parameter lists.
// Local type alias avoids importing transpiler (wrong dependency direction).
type SignalIndex = map[string][]ast.Param

// CheckOptions configures semantic validation.
type CheckOptions struct {
	FilePath    string
	SignalIndex SignalIndex
}

// CheckFile validates an AST and returns diagnostics for beginner mistakes.
func CheckFile(file ast.File, opts CheckOptions) []diag.Diagnostic {
	ctx := &checkCtx{
		file:     file,
		opts:     opts,
		filePath: span.FileID(opts.FilePath),
	}
	ctx.checkScripts()
	ctx.checkComponents()
	ctx.checkSystems()
	return ctx.diags
}

type checkCtx struct {
	file     ast.File
	opts     CheckOptions
	filePath span.FileID
	diags    []diag.Diagnostic
}

func (c *checkCtx) add(d diag.Diagnostic) {
	c.diags = append(c.diags, d)
}

func (c *checkCtx) span(line int) span.Span {
	return span.Span{
		File:  c.filePath,
		Start: span.Point{Line: line, Column: 1},
	}
}

func (c *checkCtx) checkScripts() {
	seen := map[string]int{}
	for _, script := range c.file.Scripts {
		if prev, ok := seen[script.Name]; ok {
			c.add(duplicateScript(script.Name, c.span(script.Line), prev))
		}
		seen[script.Name] = script.Line
		c.checkScript(script)
	}
}

func (c *checkCtx) checkScript(script ast.Script) {
	c.checkDuplicateFields(script)
	c.checkHandlers(script)
	c.checkStates(script)
	c.checkConnects(script)
	c.checkWatches(script)
	c.checkEmitArgCounts(script)
	c.checkEmpty(script)
}

// Stubs — replaced by real implementations in rules.go (Tasks 3-6).
func (c *checkCtx) checkDuplicateFields(_ ast.Script) {}
func (c *checkCtx) checkHandlers(_ ast.Script)        {}
func (c *checkCtx) checkStates(_ ast.Script)           {}
func (c *checkCtx) checkConnects(_ ast.Script)         {}
func (c *checkCtx) checkWatches(_ ast.Script)          {}
func (c *checkCtx) checkEmitArgCounts(_ ast.Script)    {}
func (c *checkCtx) checkEmpty(_ ast.Script)            {}
func (c *checkCtx) checkComponents()                   {}
func (c *checkCtx) checkSystems()                      {}
