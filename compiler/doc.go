// Package compiler contains the long-term Fyx compiler pipeline.
//
// The grammar and parser foundation live in grammar/, grammargen, and
// gotreesitter. The packages under compiler/ are the layers that turn parsed
// .fyx source into diagnostics, lowered semantics, and generated Rust output.
package compiler
