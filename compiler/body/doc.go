// Package body is the dedicated subsystem for handler-body compilation.
//
// It is where body-local Fyx sugar such as dt, self.node, signal emission, and
// ECS shorthands should be structurally lowered instead of spread across
// general-purpose transpiler code.
package body
