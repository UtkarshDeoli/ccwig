// Package object defines the runtime values of the CCWIG interpreter and the
// environment that maps names to them.
package object

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// Type names a runtime value's kind.
type Type string

// The runtime types.
const (
	INTEGER  = "INTEGER"
	FLOAT    = "FLOAT"
	STRING   = "STRING"
	BOOLEAN  = "BOOLEAN"
	NULL     = "NULL"
	RETURN   = "RETURN_VALUE"
	FUNCTION = "FUNCTION"
	BUILTIN  = "BUILTIN"
	ARRAY    = "ARRAY"
	ERROR    = "ERROR"
)

// Object is any runtime value.
type Object interface {
	// Type reports the value's kind.
	Type() Type
	// Inspect renders the value for display by `print` and the REPL.
	Inspect() string
}

// Integer is a 64-bit signed whole number.
type Integer struct {
	Value int64
}

func (i *Integer) Type() Type      { return INTEGER }
func (i *Integer) Inspect() string { return strconv.FormatInt(i.Value, 10) }

// Float is a 64-bit IEEE-754 number.
type Float struct {
	Value float64
}

func (f *Float) Type() Type      { return FLOAT }
func (f *Float) Inspect() string { return strconv.FormatFloat(f.Value, 'g', -1, 64) }

// String is a sequence of Unicode code points.
type String struct {
	Value string
}

func (s *String) Type() Type      { return STRING }
func (s *String) Inspect() string { return s.Value }

// Boolean is true or false.
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() Type      { return BOOLEAN }
func (b *Boolean) Inspect() string { return strconv.FormatBool(b.Value) }

// Null is the absence of a value.
type Null struct{}

func (n *Null) Type() Type      { return NULL }
func (n *Null) Inspect() string { return "null" }

// ReturnValue wraps a value travelling out of a function body. It never
// escapes to user code: the evaluator unwraps it at the call boundary.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() Type      { return RETURN }
func (rv *ReturnValue) Inspect() string { return rv.Value.Inspect() }

// Function is a user-defined function together with the environment it
// closed over.
type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() Type { return FUNCTION }
func (f *Function) Inspect() string {
	params := make([]string, 0, len(f.Parameters))
	for _, p := range f.Parameters {
		params = append(params, p.Value)
	}
	return "fn(" + strings.Join(params, ", ") + ") " + f.Body.String()
}

// Builtin is a function implemented in Go. Fn receives the evaluated
// arguments in order and returns either a value or an *Error.
type Builtin struct {
	Name string
	Fn   func(args []Object) Object
}

func (b *Builtin) Type() Type      { return BUILTIN }
func (b *Builtin) Inspect() string { return "builtin function: " + b.Name }

// Array is an ordered, mutable sequence of values.
type Array struct {
	Elements []Object
}

func (a *Array) Type() Type { return ARRAY }
func (a *Array) Inspect() string {
	parts := make([]string, 0, len(a.Elements))
	for _, e := range a.Elements {
		parts = append(parts, e.Inspect())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// Error is a runtime failure. Errors are values so that they can be returned
// from builtins and propagated by the evaluator; they halt execution when
// they reach the top level.
type Error struct {
	Message string
	Pos     token.Position
}

func (e *Error) Type() Type      { return ERROR }
func (e *Error) Inspect() string { return "ERROR: " + e.Message }

// NewError builds an error value at the given position.
func NewError(pos token.Position, format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), Pos: pos}
}

// ------------------------------------------------------------------ environment

// Environment is a lexical scope: a set of name bindings plus a link to the
// scope that encloses it.
type Environment struct {
	store map[string]Object
	outer *Environment
}

// NewEnvironment returns an empty top-level scope.
func NewEnvironment() *Environment {
	return &Environment{store: map[string]Object{}}
}

// NewEnclosedEnvironment returns an empty scope nested inside outer.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	return &Environment{store: map[string]Object{}, outer: outer}
}

// Get looks a name up in this scope and then in enclosing scopes.
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return obj, ok
}

// Set binds name in this scope, shadowing any outer binding of the same name.
// It is used by `let`.
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Assign updates the nearest existing binding of name, searching outwards
// through enclosing scopes. If no binding exists anywhere it creates one in
// this scope. It is used by `=`.
//
// Searching outwards is what makes mutation work from inside a block: a loop
// body evaluates in a fresh child scope, so without this an assignment such
// as `i = i + 1` would shadow the loop variable and the loop would never
// terminate.
func (e *Environment) Assign(name string, val Object) Object {
	for env := e; env != nil; env = env.outer {
		if _, ok := env.store[name]; ok {
			env.store[name] = val
			return val
		}
	}
	e.store[name] = val
	return val
}
