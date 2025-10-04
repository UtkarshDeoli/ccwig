// Package ccwig is the root of the CCWIG project: a small C-like language
// implemented as a tree-walking interpreter in Go.
//
// The implementation is split into subpackages that follow the phases of the
// interpreter:
//
//	internal/token      lexical tokens and source positions
//	internal/lexer      source text to tokens
//	internal/ast        syntax tree nodes
//	internal/parser     tokens to syntax tree
//	internal/object     runtime values and scopes
//	internal/evaluator  syntax tree to values
//	internal/interp     the three phases together, plus the REPL
//	cmd/ccwig           the command line interface
//
// This package holds no code of its own. It exists so that the end-to-end
// tests in integration_test.go have a home, and so that the module has a
// documented root.
package ccwig
