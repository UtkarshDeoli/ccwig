// Package evaluator walks the AST and computes its result.
//
// Evaluation is a tree-walking interpreter over the values in package object.
// Errors are ordinary values that propagate upwards: any operation given an
// *Error returns it unchanged, so the first failure aborts the whole program
// rather than being silently swallowed by a later statement.
package evaluator

import (
	"bufio"
	"fmt"
	"io"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/sockets"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// DefaultMaxSteps bounds how many AST nodes a single program may evaluate.
// It exists so that a runaway `while true {}` fails with a diagnosable error
// instead of hanging the process.
const DefaultMaxSteps = 50_000_000

// DefaultMaxDepth bounds how deeply user function calls may nest.
//
// Evaluation recurses once per nested call, so the depth a program can reach
// is bounded by the Go stack, and exhausting the stack is a fatal error that
// recover cannot catch. This limit stops a runaway recursion while there is
// still stack left to report the problem. It is deliberately far larger than
// any hand-written program needs, because reaching it almost always means the
// program is wrong rather than merely deep.
const DefaultMaxDepth = 10_000

// Builtin names, kept in one place so the docs and error messages agree.
const (
	builtinLen   = "len"
	builtinPush  = "push"
	builtinFill  = "fill"
	builtinFirst = "first"
	builtinLast  = "last"
	builtinRest  = "rest"
	builtinType  = "type"
	builtinStr   = "str"
	builtinPrint = "print"
	builtinClear = "clear"
	builtinSleep = "sleep"
)

// BuiltinNames lists the built-in functions in documentation order.
var BuiltinNames = []string{
	builtinPrint, builtinLen, builtinPush, builtinFill, builtinFirst, builtinLast, builtinRest, builtinType, builtinStr,
	builtinOrd, builtinChr, builtinSubstr, builtinSplit, builtinJoin, builtinFind, builtinUpper, builtinLower,
	builtinClear, builtinSleep,
	builtinArgs, builtinReadLine, builtinReadAll, builtinReadFile, builtinWriteFile, builtinTime, builtinExit,
	builtinNetListen, builtinNetAccept, builtinNetConnect, builtinNetSend, builtinNetRecv,
	builtinNetRecvLine, builtinNetClose, builtinNetSetTimeout, builtinNetAddr, builtinNetError,
}

// clearScreen is what `clear` writes: erase the whole screen, then move the
// cursor back to the top-left corner so the next frame starts there.
const clearScreen = "\x1b[2J\x1b[H"

// Evaluator evaluates parsed programs. It is not safe for concurrent use.
type Evaluator struct {
	// Out receives everything written by `print`.
	Out io.Writer
	// MaxSteps bounds the number of AST nodes evaluated per program run.
	// Zero means DefaultMaxSteps.
	MaxSteps int
	// MaxDepth bounds how deeply user function calls may nest. Zero means
	// DefaultMaxDepth.
	MaxDepth int
	// In is the standard input that `readLine` and `readAll` read. A nil In
	// means there is no input to read, and asking for some is an error.
	In io.Reader
	// Args is what `args()` returns: the arguments the program was given,
	// without the name of the interpreter or the file it came from.
	Args []string

	builtins map[string]*object.Builtin
	steps    int
	depth    int
	// stdin is the buffered reader over In, created on first use, and
	// stdinFor is the In it was created for so that replacing In on a reused
	// evaluator starts reading the new one rather than the old.
	stdin    *bufio.Reader
	stdinFor io.Reader
	// socks holds this evaluator's open sockets, created on first use.
	socks *sockets.Registry
}

// New returns an Evaluator writing program output to out.
func New(out io.Writer) *Evaluator {
	e := &Evaluator{Out: out, MaxSteps: DefaultMaxSteps, MaxDepth: DefaultMaxDepth}
	e.builtins = e.newBuiltins()
	return e
}

// ResetCallDepth returns the call-depth counter to zero.
//
// It exists for the one case where evaluation stops without unwinding
// normally: `exit` panics out of the evaluator, skipping the depth decrement
// that every other return path performs, so an interpreter that survives the
// exit would otherwise believe it was still several calls deep.
func (e *Evaluator) ResetCallDepth() { e.depth = 0 }

// Eval evaluates node in env and returns the resulting value.
func (e *Evaluator) Eval(node ast.Node, env *object.Environment) object.Object {
	if node == nil {
		return object.NullValue()
	}

	if e.spendStep() {
		return e.stepLimitError(node.Pos())
	}

	switch node := node.(type) {
	case *ast.Program:
		return e.evalProgram(node, env)

	case *ast.ExpressionStatement:
		return e.Eval(node.Expression, env)

	case *ast.BlockStatement:
		return e.evalBlockStatement(node, env)

	case *ast.LetStatement:
		val := e.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
		return nil

	case *ast.FunctionStatement:
		// A named declaration is sugar for a `let` binding, which is what
		// makes recursion work: the function closes over env and env gains
		// the binding before the body ever runs.
		env.Set(node.Name.Value, &object.Function{
			Parameters: node.Function.Parameters,
			Body:       node.Function.Body,
			Env:        env,
		})
		return nil

	case *ast.ReturnStatement:
		if node.ReturnValue == nil {
			return &object.ReturnValue{Value: object.NullValue()}
		}
		val := e.Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.WhileStatement:
		return e.evalWhileStatement(node, env)

	case *ast.ForStatement:
		return e.evalForStatement(node, env)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}

	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.BooleanLiteral:
		return object.BooleanOf(node.Value)

	case *ast.NullLiteral:
		return object.NullValue()

	case *ast.Identifier:
		return e.evalIdentifier(node, env)

	case *ast.PrefixExpression:
		right := e.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return e.evalPrefixExpression(node.Token, node.Operator, right)

	case *ast.InfixExpression:
		return e.evalInfixExpression(node, env)

	case *ast.IfExpression:
		return e.evalIfExpression(node, env)

	case *ast.FunctionLiteral:
		return &object.Function{
			Parameters: node.Parameters,
			Body:       node.Body,
			Env:        env,
		}

	case *ast.CallExpression:
		function := e.Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := e.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return e.applyFunction(node.Token, function, args)

	case *ast.ArrayLiteral:
		elements := e.evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}

	case *ast.IndexExpression:
		left := e.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := e.Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return e.evalIndexExpression(node.Token, left, index)
	}

	return object.NewError(node.Pos(), "unsupported node type %T", node)
}

// ------------------------------------------------------------------- programs

// spendStep accounts for one unit of work, reporting whether the step budget is
// now spent.
//
// Evaluation is metered here rather than only where an AST node is visited,
// because visiting nodes is not the only way to loop forever. `for ;; { }` has
// no condition to evaluate and a body that evaluates nothing at all, so a
// loop that only counted nodes would run until the process was killed. Counting
// the iteration itself closes that hole for every loop, whatever its body does.
//
// Counting is kept apart from reporting so that the common case is an
// increment and a comparison. The position is only needed for the message, and
// asking the node for it on every step costs more than the step itself.
func (e *Evaluator) spendStep() bool {
	e.steps++
	return e.MaxSteps > 0 && e.steps > e.MaxSteps
}

// stepLimitError reports an exhausted step budget at pos.
func (e *Evaluator) stepLimitError(pos token.Position) *object.Error {
	return object.NewError(pos, "step limit of %d exceeded (possible infinite loop)", e.MaxSteps)
}

// evalProgram evaluates each statement in order. Evaluation stops at the
// first error or return value.
func (e *Evaluator) evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object = object.NullValue()

	for _, stmt := range program.Statements {
		value := e.Eval(stmt, env)
		if value == nil {
			// A statement that produces no value (a `let`, for instance)
			// leaves the previous result alone.
			continue
		}

		switch value := value.(type) {
		case *object.ReturnValue:
			return value.Value
		case *object.Error:
			return value
		default:
			result = value
		}
	}

	return result
}

// evalBlockStatement evaluates a block in a fresh child scope, so that `let`
// inside the block does not leak out.
func (e *Evaluator) evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	inner := object.NewEnclosedEnvironment(env)

	for _, stmt := range block.Statements {
		result = e.Eval(stmt, inner)
		if result == nil {
			continue
		}
		if result.Type() == object.RETURN || result.Type() == object.ERROR {
			return result
		}
	}

	return result
}

// ------------------------------------------------------------------ loops

// evalWhileStatement re-evaluates the condition in env (not in the body scope)
// and gives the body a fresh child scope on every iteration.
func (e *Evaluator) evalWhileStatement(node *ast.WhileStatement, env *object.Environment) object.Object {
	for {
		if e.spendStep() {
			return e.stepLimitError(node.Token.Pos)
		}

		condition := e.Eval(node.Condition, env)
		if isError(condition) {
			return condition
		}
		if !isTruthy(condition) {
			return object.NullValue()
		}

		result := e.evalLoopBody(node.Body, env)
		if result != nil && (result.Type() == object.RETURN || result.Type() == object.ERROR) {
			return result
		}
	}
}

// evalForStatement runs init once in a scope of its own, then evaluates the
// condition and the init/post clauses there while each body iteration gets a
// fresh child scope.
func (e *Evaluator) evalForStatement(node *ast.ForStatement, env *object.Environment) object.Object {
	loopEnv := object.NewEnclosedEnvironment(env)

	if node.Init != nil {
		if result := e.Eval(node.Init, loopEnv); isError(result) {
			return result
		}
	}

	for {
		if e.spendStep() {
			return e.stepLimitError(node.Token.Pos)
		}

		if node.Condition != nil {
			condition := e.Eval(node.Condition, loopEnv)
			if isError(condition) {
				return condition
			}
			if !isTruthy(condition) {
				return object.NullValue()
			}
		}

		result := e.evalLoopBody(node.Body, loopEnv)
		if result != nil && (result.Type() == object.RETURN || result.Type() == object.ERROR) {
			return result
		}

		if node.Post != nil {
			if result := e.Eval(node.Post, loopEnv); isError(result) {
				return result
			}
		}
	}
}

// evalLoopBody evaluates a loop body in a fresh child scope, returning any
// return value or error that must abort the loop.
func (e *Evaluator) evalLoopBody(body *ast.BlockStatement, env *object.Environment) object.Object {
	result := e.evalBlockStatement(body, env)
	if result == nil {
		return nil
	}
	if result.Type() == object.RETURN || result.Type() == object.ERROR {
		return result
	}
	return nil
}

// ---------------------------------------------------------- identifiers

func (e *Evaluator) evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := e.builtins[node.Value]; ok {
		return builtin
	}
	return object.NewError(node.Token.Pos, "identifier not found: %s", node.Value)
}

// ------------------------------------------------------------- conditionals

func (e *Evaluator) evalIfExpression(node *ast.IfExpression, env *object.Environment) object.Object {
	condition := e.Eval(node.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		// The consequence is evaluated in a new scope; a return value or
		// error must escape it.
		return unwrapBlockResult(e.evalBlockStatement(node.Consequence, env))
	}

	switch alt := node.Alternative.(type) {
	case nil:
		return object.NullValue()
	case *ast.BlockStatement:
		return unwrapBlockResult(e.evalBlockStatement(alt, env))
	case *ast.IfExpression:
		return e.Eval(alt, env)
	default:
		return object.NewError(node.Token.Pos, "unsupported else branch type %T", node.Alternative)
	}
}

// unwrapBlockResult converts a nil block result (a block ending in a `let`)
// into null.
func unwrapBlockResult(result object.Object) object.Object {
	if result == nil {
		return object.NullValue()
	}
	return result
}

// ------------------------------------------------------------- expressions

func (e *Evaluator) evalExpressions(exprs []ast.Expression, env *object.Environment) []object.Object {
	result := make([]object.Object, 0, len(exprs))

	for _, expr := range exprs {
		evaluated := e.Eval(expr, env)
		if isError(evaluated) {
			// Returning a single-element slice is the convention callers
			// use to detect that evaluation failed.
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func (e *Evaluator) evalPrefixExpression(tok token.Token, operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return nativeToBoolean(!isTruthy(right))
	case "-":
		switch operand := right.(type) {
		case *object.Integer:
			return &object.Integer{Value: -operand.Value}
		case *object.Float:
			return &object.Float{Value: -operand.Value}
		default:
			return object.NewError(tok.Pos, "unknown operator: -%s", right.Type())
		}
	case "~":
		// Bitwise complement, which C spells the same way and Go spells ^.
		// It is defined on integers only, so a float is reported rather than
		// converted.
		switch operand := right.(type) {
		case *object.Integer:
			return &object.Integer{Value: ^operand.Value}
		default:
			return object.NewError(tok.Pos, "unknown operator: ~%s", right.Type())
		}
	default:
		return object.NewError(tok.Pos, "unknown operator: %s%s", operator, right.Type())
	}
}

func (e *Evaluator) evalInfixExpression(node *ast.InfixExpression, env *object.Environment) object.Object {
	if node.Operator == "=" {
		return e.evalAssignment(node, env)
	}

	left := e.Eval(node.Left, env)
	if isError(left) {
		return left
	}

	// `&&` and `||` short-circuit: the right operand is not evaluated when
	// the left one already decides the result. This makes a guard such as
	// `i < len(xs) && xs[i] != null` safe to write.
	switch node.Operator {
	case "&&":
		if !isTruthy(left) {
			return nativeToBoolean(false)
		}
		return e.evalBooleanOperand(node.Token, node.Right, env)
	case "||":
		if isTruthy(left) {
			return nativeToBoolean(true)
		}
		return e.evalBooleanOperand(node.Token, node.Right, env)
	}

	right := e.Eval(node.Right, env)
	if isError(right) {
		return right
	}

	return e.evalInfixOperation(node.Token, node.Operator, left, right)
}

// evalBooleanOperand evaluates the right-hand side of a logical operator and
// reduces it to a boolean, yielding an error if it fails.
func (e *Evaluator) evalBooleanOperand(tok token.Token, operand ast.Expression, env *object.Environment) object.Object {
	value := e.Eval(operand, env)
	if isError(value) {
		return value
	}
	if value == nil {
		return object.NewError(tok.Pos, "operand of %s produced no value", tok.Literal)
	}
	return nativeToBoolean(isTruthy(value))
}

func (e *Evaluator) evalInfixOperation(tok token.Token, operator string, left, right object.Object) object.Object {
	// Equality is defined across every pair of types: comparing different
	// types yields false rather than an error, so `x == null` is always safe.
	switch operator {
	case "==":
		return nativeToBoolean(objectsEqual(left, right))
	case "!=":
		return nativeToBoolean(!objectsEqual(left, right))
	}

	// Dispatch on the concrete types rather than on Type() names. This is the
	// hottest path in the evaluator, and a type switch on a handful of pointer
	// types is a few comparisons, where asking each operand for its type name
	// and comparing strings is several.
	switch l := left.(type) {
	case *object.Integer:
		switch r := right.(type) {
		case *object.Integer:
			return evalIntegerInfix(tok, operator, l, r)
		case *object.Float:
			return evalFloatInfix(tok, operator, l, r)
		}

	case *object.Float:
		switch r := right.(type) {
		case *object.Integer:
			return evalFloatInfix(tok, operator, l, r)
		case *object.Float:
			return evalFloatInfix(tok, operator, l, r)
		}

	case *object.String:
		if r, ok := right.(*object.String); ok {
			return evalStringInfix(tok, operator, l, r)
		}
	}

	// Nothing matched, so this is a failure. Naming the types is only needed
	// here, which keeps it off the successful path.
	return e.infixTypeError(tok, operator, left, right)
}

// infixTypeError explains why an operator could not be applied.
//
// Mixing types that can never combine is reported as a mismatch, which names
// both types; an operator that is simply undefined for a pair of equal types —
// `%` on two FLOATs, say — is reported as an unknown operator.
func (e *Evaluator) infixTypeError(tok token.Token, operator string, left, right object.Object) object.Object {
	if left.Type() != right.Type() {
		return object.NewError(tok.Pos, "type mismatch: %s %s %s",
			left.Type(), operator, right.Type())
	}
	return object.NewError(tok.Pos, "unknown operator: %s %s %s",
		left.Type(), operator, right.Type())
}

func evalIntegerInfix(tok token.Token, operator string, left, right *object.Integer) object.Object {
	switch operator {
	case "+":
		return &object.Integer{Value: left.Value + right.Value}
	case "-":
		return &object.Integer{Value: left.Value - right.Value}
	case "*":
		return &object.Integer{Value: left.Value * right.Value}
	case "/":
		if right.Value == 0 {
			return object.NewError(tok.Pos, "division by zero")
		}
		return &object.Integer{Value: left.Value / right.Value}
	case "%":
		if right.Value == 0 {
			return object.NewError(tok.Pos, "modulo by zero")
		}
		return &object.Integer{Value: left.Value % right.Value}
	case "&":
		return &object.Integer{Value: left.Value & right.Value}
	case "|":
		return &object.Integer{Value: left.Value | right.Value}
	case "^":
		return &object.Integer{Value: left.Value ^ right.Value}
	case "<<":
		return shiftInteger(tok, "<<", left.Value, right.Value)
	case ">>":
		return shiftInteger(tok, ">>", left.Value, right.Value)
	case "<":
		return nativeToBoolean(left.Value < right.Value)
	case ">":
		return nativeToBoolean(left.Value > right.Value)
	case "<=":
		return nativeToBoolean(left.Value <= right.Value)
	case ">=":
		return nativeToBoolean(left.Value >= right.Value)
	default:
		return object.NewError(tok.Pos, "unknown operator: INTEGER %s INTEGER", operator)
	}
}

// shiftInteger applies `<<` or `>>` to two integers.
//
// C leaves a shift by a negative count, or by a count at or beyond the width
// of the type, undefined. Neither is a panic here: a negative count is
// reported as an error, and an oversized count follows Go, where shifting an
// int64 by 64 or more yields zero for `<<` and the sign for `>>`. Defined
// behaviour beats undefined behaviour even when the definition is arbitrary.
func shiftInteger(tok token.Token, operator string, value, count int64) object.Object {
	if count < 0 {
		return object.NewError(tok.Pos, "shift by a negative amount: %d", count)
	}
	// The conversion is safe: count is non-negative, and a count of 64 or more
	// is defined rather than a panic.
	shift := uint64(count)
	if operator == "<<" {
		return &object.Integer{Value: value << shift}
	}
	return &object.Integer{Value: value >> shift}
}

// evalFloatInfix handles an operation where at least one operand is a FLOAT.
// Mixed INTEGER/FLOAT operands are promoted, so the original objects are kept
// around for error messages.
func evalFloatInfix(tok token.Token, operator string, left, right object.Object) object.Object {
	l, r := toFloat(left), toFloat(right)

	switch operator {
	case "+":
		return &object.Float{Value: l + r}
	case "-":
		return &object.Float{Value: l - r}
	case "*":
		return &object.Float{Value: l * r}
	case "/":
		if r == 0 {
			return object.NewError(tok.Pos, "division by zero")
		}
		return &object.Float{Value: l / r}
	case "<":
		return nativeToBoolean(l < r)
	case ">":
		return nativeToBoolean(l > r)
	case "<=":
		return nativeToBoolean(l <= r)
	case ">=":
		return nativeToBoolean(l >= r)
	default:
		// Reached for `%`, which is integer-only.
		return object.NewError(tok.Pos, "unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfix(tok token.Token, operator string, left, right *object.String) object.Object {
	switch operator {
	case "+":
		return &object.String{Value: left.Value + right.Value}
	case "<":
		return nativeToBoolean(left.Value < right.Value)
	case ">":
		return nativeToBoolean(left.Value > right.Value)
	case "<=":
		return nativeToBoolean(left.Value <= right.Value)
	case ">=":
		return nativeToBoolean(left.Value >= right.Value)
	default:
		return object.NewError(tok.Pos, "unknown operator: STRING %s STRING", operator)
	}
}

// evalAssignment handles `name = value` and `array[index] = value`.
func (e *Evaluator) evalAssignment(node *ast.InfixExpression, env *object.Environment) object.Object {
	value := e.Eval(node.Right, env)
	if isError(value) {
		return value
	}

	switch target := node.Left.(type) {
	case *ast.Identifier:
		return env.Assign(target.Value, value)

	case *ast.IndexExpression:
		return e.evalIndexAssignment(node.Token, target, value, env)

	default:
		return object.NewError(node.Token.Pos, "cannot assign to %s", node.Left.String())
	}
}

func (e *Evaluator) evalIndexAssignment(tok token.Token, target *ast.IndexExpression, value object.Object, env *object.Environment) object.Object {
	left := e.Eval(target.Left, env)
	if isError(left) {
		return left
	}

	arr, ok := left.(*object.Array)
	if !ok {
		if _, isString := left.(*object.String); isString {
			// s[i] reads a character; there is no s[i] = c, because a string
			// is immutable and every index into it would have to be rewritten
			// to keep `len` honest.
			return object.NewError(tok.Pos, "cannot assign to a character in a string; strings are immutable")
		}
		return object.NewError(tok.Pos, "index operator not supported: %s", left.Type())
	}

	index := e.Eval(target.Index, env)
	if isError(index) {
		return index
	}

	idx, ok := index.(*object.Integer)
	if !ok {
		return object.NewError(tok.Pos, "index must be an integer, got %s", index.Type())
	}

	if idx.Value < 0 || idx.Value >= int64(len(arr.Elements)) {
		return object.NewError(tok.Pos, "index out of range: %d (array length %d)", idx.Value, len(arr.Elements))
	}

	arr.Elements[idx.Value] = value
	return value
}

func (e *Evaluator) evalIndexExpression(tok token.Token, left, index object.Object) object.Object {
	idx, ok := index.(*object.Integer)
	if !ok {
		return object.NewError(tok.Pos, "index must be an integer, got %s", index.Type())
	}

	switch container := left.(type) {
	case *object.Array:
		// Reading outside the array yields null rather than failing, so callers
		// can test for an absent element with `== null`.
		if idx.Value < 0 || idx.Value >= int64(len(container.Elements)) {
			return object.NullValue()
		}
		return container.Elements[idx.Value]

	case *object.String:
		// A character of a string, as a one-character string. The index counts
		// characters to match `len`, so `s[i]` for the last i is the last
		// character even when the text is not ASCII.
		runes := []rune(container.Value)
		if idx.Value < 0 || idx.Value >= int64(len(runes)) {
			return object.NullValue()
		}
		return &object.String{Value: string(runes[idx.Value])}

	default:
		return object.NewError(tok.Pos, "index operator not supported: %s", left.Type())
	}
}

// ---------------------------------------------------------------- functions

func (e *Evaluator) applyFunction(tok token.Token, fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		return e.applyUserFunction(tok, fn, args)

	case *object.Builtin:
		result := fn.Fn(args)
		// Builtins do not know where they were called from, so fill in the
		// call site for any error they produced.
		if errObj, ok := result.(*object.Error); ok && errObj.Pos.Line == 0 {
			errObj.Pos = tok.Pos
		}
		return result

	default:
		return object.NewError(tok.Pos, "not a function: %s", fn.Type())
	}
}

// applyUserFunction calls a function defined in CCWIG source.
//
// This is the one place where evaluation recurses without an AST to bound it,
// so it is where the call depth is metered. The counter is unwound by hand
// rather than with defer, which would be a cost on every call.
func (e *Evaluator) applyUserFunction(tok token.Token, fn *object.Function, args []object.Object) object.Object {
	if len(args) != len(fn.Parameters) {
		return object.NewError(tok.Pos, "wrong number of arguments: want %d, got %d", len(fn.Parameters), len(args))
	}

	limit := e.MaxDepth
	if limit <= 0 {
		limit = DefaultMaxDepth
	}
	if e.depth >= limit {
		return object.NewError(tok.Pos,
			"maximum call depth of %d exceeded (possible infinite recursion)", limit)
	}

	// Parameters are bound in a scope enclosed by the function's
	// definition environment, which is what makes closures work.
	callEnv := object.NewEnclosedEnvironment(fn.Env)
	for i, param := range fn.Parameters {
		callEnv.Set(param.Value, args[i])
	}

	e.depth++
	result := e.Eval(fn.Body, callEnv)
	e.depth--

	return unwrapReturn(result)
}

// unwrapReturn strips a ReturnValue produced by a function body. A body that
// falls off the end yields null.
func unwrapReturn(result object.Object) object.Object {
	switch result := result.(type) {
	case nil:
		return object.NullValue()
	case *object.ReturnValue:
		return result.Value
	default:
		return result
	}
}

// ------------------------------------------------------------------ builtins

func (e *Evaluator) newBuiltins() map[string]*object.Builtin {
	builtins := map[string]*object.Builtin{
		builtinPrint: {
			Name: builtinPrint,
			Fn: func(args []object.Object) object.Object {
				parts := make([]string, 0, len(args))
				for _, a := range args {
					parts = append(parts, a.Inspect())
				}
				fmt.Fprintln(e.Out, strings.Join(parts, " "))
				return object.NullValue()
			},
		},

		builtinClear: {
			Name: builtinClear,
			// `clear` erases the screen and homes the cursor, so that the next
			// thing written starts a fresh frame. It is terminal control rather
			// than output: nothing else is written, not even a newline.
			//
			// It writes the sequence unconditionally, without checking whether
			// the output is really a terminal. A builtin that quietly did
			// nothing when it guessed otherwise would be harder to reason
			// about than one that does exactly what it says, and the escape
			// bytes are legible in a pager such as `less -R`.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinClear, args, 0, token.Position{}); err != nil {
					return err
				}
				fmt.Fprint(e.Out, clearScreen)
				return object.NullValue()
			},
		},

		builtinSleep: {
			Name: builtinSleep,
			// `sleep` pauses for a number of milliseconds. It is what makes an
			// animation run at a speed a person can watch, and it is the only
			// builtin whose cost is measured in wall-clock time rather than in
			// work done.
			//
			// A duration must be a whole number of milliseconds and must not be
			// negative: there is no such thing as a pause of minus a second,
			// and reporting that is more useful than quietly treating it as
			// none. Zero is allowed and returns at once, which is the natural
			// way to make a pause conditional.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinSleep, args, 1, token.Position{}); err != nil {
					return err
				}
				ms, ok := args[0].(*object.Integer)
				if !ok {
					return object.NewError(token.Position{}, "argument to `sleep` must be an integer, got %s", args[0].Type())
				}
				if ms.Value < 0 {
					return object.NewError(token.Position{}, "argument to `sleep` must not be negative, got %d", ms.Value)
				}
				time.Sleep(time.Duration(ms.Value) * time.Millisecond)
				return object.NullValue()
			},
		},

		builtinLen: {
			Name: builtinLen,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinLen, args, 1, token.Position{}); err != nil {
					return err
				}
				switch arg := args[0].(type) {
				case *object.String:
					// Count characters, not bytes, so that non-ASCII text
					// behaves the way a reader expects.
					return &object.Integer{Value: int64(len([]rune(arg.Value)))}
				case *object.Array:
					return &object.Integer{Value: int64(len(arg.Elements))}
				default:
					return object.NewError(token.Position{}, "argument to `len` not supported, got %s", args[0].Type())
				}
			},
		},

		builtinPush: {
			Name: builtinPush,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinPush, args, 2, token.Position{}); err != nil {
					return err
				}
				arr, ok := args[0].(*object.Array)
				if !ok {
					return object.NewError(token.Position{}, "argument to `push` must be an array, got %s", args[0].Type())
				}

				// Copy, so that the caller's array is left alone and `push`
				// can be used to build a new value in a loop.
				elements := make([]object.Object, len(arr.Elements), len(arr.Elements)+1)
				copy(elements, arr.Elements)
				elements = append(elements, args[1])

				return &object.Array{Elements: elements}
			},
		},

		builtinFill: {
			Name: builtinFill,
			// `fill` makes an array of n copies of a value. It is here because
			// `push` copies the whole array on every call, so building an array
			// a push at a time copies about n*n/2 elements: fine for the
			// handful of elements most programs want, hopeless for the hundred
			// thousand a large grid needs. This allocates once.
			//
			// The value is repeated, not copied, so an array argument is
			// aliased into every slot: fill(rows, fill(cols, 0)) is one row
			// shared rows times over, and writing to one cell of it writes to
			// all of them. A grid of genuinely separate rows needs a fresh
			// fill per row, pushed on in a loop.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinFill, args, 2, token.Position{}); err != nil {
					return err
				}
				n, ok := args[0].(*object.Integer)
				if !ok {
					return object.NewError(token.Position{}, "argument to `fill` must be an integer, got %s", args[0].Type())
				}
				if n.Value < 0 {
					return object.NewError(token.Position{}, "argument to `fill` must not be negative, got %d", n.Value)
				}

				elements := make([]object.Object, n.Value)
				for i := range elements {
					elements[i] = args[1]
				}

				return &object.Array{Elements: elements}
			},
		},

		builtinFirst: {
			Name: builtinFirst,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinFirst, args, 1, token.Position{}); err != nil {
					return err
				}
				arr, ok := args[0].(*object.Array)
				if !ok {
					return object.NewError(token.Position{}, "argument to `first` must be an array, got %s", args[0].Type())
				}
				if len(arr.Elements) == 0 {
					return object.NullValue()
				}
				return arr.Elements[0]
			},
		},

		builtinLast: {
			Name: builtinLast,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinLast, args, 1, token.Position{}); err != nil {
					return err
				}
				arr, ok := args[0].(*object.Array)
				if !ok {
					return object.NewError(token.Position{}, "argument to `last` must be an array, got %s", args[0].Type())
				}
				if len(arr.Elements) == 0 {
					return object.NullValue()
				}
				return arr.Elements[len(arr.Elements)-1]
			},
		},

		builtinRest: {
			Name: builtinRest,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinRest, args, 1, token.Position{}); err != nil {
					return err
				}
				arr, ok := args[0].(*object.Array)
				if !ok {
					return object.NewError(token.Position{}, "argument to `rest` must be an array, got %s", args[0].Type())
				}
				if len(arr.Elements) == 0 {
					return object.NullValue()
				}
				rest := make([]object.Object, len(arr.Elements)-1)
				copy(rest, arr.Elements[1:])
				return &object.Array{Elements: rest}
			},
		},

		builtinType: {
			Name: builtinType,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinType, args, 1, token.Position{}); err != nil {
					return err
				}
				return &object.String{Value: string(args[0].Type())}
			},
		},

		builtinStr: {
			Name: builtinStr,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinStr, args, 1, token.Position{}); err != nil {
					return err
				}
				return &object.String{Value: args[0].Inspect()}
			},
		},
	}

	maps.Copy(builtins, e.stringBuiltins())
	maps.Copy(builtins, e.ioBuiltins())
	maps.Copy(builtins, e.netBuiltins())

	return builtins
}

// expectArgs checks a builtin's arity.
func expectArgs(name string, args []object.Object, want int, pos token.Position) object.Object {
	if len(args) != want {
		return object.NewError(pos, "wrong number of arguments to `%s`: want %d, got %d", name, want, len(args))
	}
	return nil
}

// ------------------------------------------------------------------- helpers

// isTruthy reports whether a value counts as true in a condition.
//
// CCWIG follows C here: false, null, integer 0 and float 0.0 are falsy.
// Everything else, including the empty string and empty arrays, is truthy.
func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	case *object.Integer:
		return obj.Value != 0
	case *object.Float:
		return obj.Value != 0
	default:
		return true
	}
}

// objectsEqual implements `==`. Values of different types are never equal, so
// this never fails.
//
// Like the arithmetic path, it dispatches on concrete types rather than on type
// names, and INTEGER compares against FLOAT numerically so that `1 == 1.0`
// holds.
func objectsEqual(left, right object.Object) bool {
	switch l := left.(type) {
	case *object.Integer:
		switch r := right.(type) {
		case *object.Integer:
			return l.Value == r.Value
		case *object.Float:
			return float64(l.Value) == r.Value
		}
		return false

	case *object.Float:
		switch r := right.(type) {
		case *object.Integer:
			return l.Value == float64(r.Value)
		case *object.Float:
			return l.Value == r.Value
		}
		return false

	case *object.String:
		r, ok := right.(*object.String)
		return ok && l.Value == r.Value

	case *object.Boolean:
		r, ok := right.(*object.Boolean)
		return ok && l.Value == r.Value

	case *object.Null:
		_, ok := right.(*object.Null)
		return ok

	default:
		// Functions, builtins and arrays compare by identity.
		return left == right
	}
}

func isError(obj object.Object) bool {
	if obj == nil {
		return false
	}
	return obj.Type() == object.ERROR
}

func toFloat(obj object.Object) float64 {
	switch obj := obj.(type) {
	case *object.Integer:
		return float64(obj.Value)
	case *object.Float:
		return obj.Value
	default:
		return math.NaN()
	}
}

func nativeToBoolean(value bool) *object.Boolean {
	return object.BooleanOf(value)
}
