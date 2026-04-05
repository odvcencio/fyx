package check

import (
	"strings"

	"github.com/odvcencio/fyx/ast"
)

func (c *checkCtx) checkDuplicateFields(script ast.Script) {
	seen := map[string]int{}
	for _, field := range script.Fields {
		if _, ok := seen[field.Name]; ok {
			c.add(duplicateField(field.Name, script.Name, c.span(field.Line)))
		}
		seen[field.Name] = field.Line

		if field.TypeExpr == "" && field.Modifier != ast.FieldDerived {
			c.add(missingFieldType(field.Name, c.span(field.Line)))
		}

		if (field.Modifier == ast.FieldNode || field.Modifier == ast.FieldNodes) && field.Default != "" {
			if !strings.HasPrefix(field.Default, "\"") {
				c.add(nodePathWithoutQuotes(field.Name, c.span(field.Line)))
			}
		}
	}
}
