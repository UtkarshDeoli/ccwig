package evaluator

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/parser"
)

// eval parses and evaluates input in a fresh environment, discarding output.
//
// This is not a host-language eval: it runs CCWIG source through this
// package's own lexer/parser/tree-walking evaluator, which is the system
// under test.
func eval(t *testing.T, input string) object.Object {
	t.Helper()
	return evalWith(t, input, New(io.Discard))
}

func evalWith(t *testing.T, input string, ev *Evaluator) object.Object {
	t.Helper()

	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parser errors for input %q:\n  %v", input, errs)
	}

	return ev.Eval(program, object.NewEnvironment())
}

// evalOutput evaluates input and returns everything written by `print`.
func evalOutput(t *testing.T, input string) string {
	t.Helper()

	var buf bytes.Buffer
	evalWith(t, input, New(&buf))

	return buf.String()
}

func assertInteger(t *testing.T, obj object.Object, want int64) {
	t.Helper()

	got, ok := obj.(*object.Integer)
	if !ok {
		t.Fatalf("expected *object.Integer, got %T (%s)", obj, obj.Inspect())
	}
	if got.Value != want {
		t.Errorf("expected %d, got %d", want, got.Value)
	}
}

func assertInspect(t *testing.T, obj object.Object, want string) {
	t.Helper()

	if got := obj.Inspect(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func assertError(t *testing.T, obj object.Object, want string) {
	t.Helper()

	errObj, ok := obj.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%s)", obj, obj.Inspect())
	}
	if !strings.Contains(errObj.Message, want) {
		t.Errorf("expected error to contain %q, got %q", want, errObj.Message)
	}
}

// ------------------------------------------------------------------ integers

func TestIntegerArithmetic(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"20 + 2 * -10", 0},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestIntegerDivisionTruncatesTowardZero(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"7 / 2", 3},
		{"-7 / 2", -3},
		{"7 % 3", 1},
		{"-7 % 3", -1},
		{"7 % -3", 1},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestDivisionAndModuloByZero(t *testing.T) {
	assertError(t, eval(t, "1 / 0"), "division by zero")
	assertError(t, eval(t, "1 % 0"), "modulo by zero")
	assertError(t, eval(t, "1.0 / 0"), "division by zero")
}

// -------------------------------------------------------------------- floats

func TestFloatArithmetic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"3.14", "3.14"},
		{"1.5 + 2.5", "4"},
		{"1.0 / 2", "0.5"},
		{"10.0 / 4", "2.5"},
		{"2.5 * 2", "5"},
	}

	for _, tt := range tests {
		assertInspect(t, eval(t, tt.input), tt.want)
	}
}

func TestIntAndFloatArePromotedForArithmetic(t *testing.T) {
	assertInspect(t, eval(t, "1 + 2.5"), "3.5")
	assertInspect(t, eval(t, "2.5 + 1"), "3.5")
	assertInspect(t, eval(t, "1 < 1.5"), "true")
}

func TestModuloIsIntegerOnly(t *testing.T) {
	assertError(t, eval(t, "5.0 % 2"), "unknown operator: FLOAT % INTEGER")
	assertError(t, eval(t, "5 % 2.0"), "unknown operator: INTEGER % FLOAT")
}

// ------------------------------------------------------------------ booleans

func TestBooleanExpressions(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"1 <= 1", true},
		{"1 >= 1", true},
		{"2 >= 3", false},
		{"2 <= 1", false},
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"true && true", true},
		{"true && false", false},
		{"false || true", true},
		{"false || false", false},
		{"1 == 1 && 2 == 2", true},
	}

	for _, tt := range tests {
		obj := eval(t, tt.input)
		got, ok := obj.(*object.Boolean)
		if !ok {
			t.Fatalf("input %q: expected *object.Boolean, got %T (%s)", tt.input, obj, obj.Inspect())
		}
		if got.Value != tt.want {
			t.Errorf("input %q: expected %t, got %t", tt.input, tt.want, got.Value)
		}
	}
}

func TestBangOperator(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
		{"!null", true},
		{"!0", true},
		{"!\"\"", false},
		{"!\"x\"", false},
	}

	for _, tt := range tests {
		obj := eval(t, tt.input)
		got, ok := obj.(*object.Boolean)
		if !ok {
			t.Fatalf("input %q: expected *object.Boolean, got %T (%s)", tt.input, obj, obj.Inspect())
		}
		if got.Value != tt.want {
			t.Errorf("input %q: expected %t, got %t", tt.input, tt.want, got.Value)
		}
	}
}

func TestNegatingNonNumbersIsAnError(t *testing.T) {
	assertError(t, eval(t, "-true"), "unknown operator: -BOOLEAN")
	assertError(t, eval(t, "-\"a\""), "unknown operator: -STRING")
}

// ---------------------------------------------------------------- truthiness

func TestTruthiness(t *testing.T) {
	// CCWIG follows C: zero and null are falsy; everything else, including
	// the empty string and empty arrays, is truthy.
	tests := []struct {
		condition string
		want      string
	}{
		{"true", "yes"},
		{"false", "no"},
		{"1", "yes"},
		{"0", "no"},
		{"-1", "yes"},
		{"0.0", "no"},
		{"\"\"", "yes"},
		{"null", "no"},
		{"[]", "yes"},
		{"[0]", "yes"},
	}

	for _, tt := range tests {
		got := strings.TrimSpace(evalOutput(t, "if "+tt.condition+" { print(\"yes\"); } else { print(\"no\"); }"))
		if got != tt.want {
			t.Errorf("condition %q: expected %q, got %q", tt.condition, tt.want, got)
		}
	}
}

func TestLogicalOperatorsShortCircuit(t *testing.T) {
	// The right operand must not be evaluated when the left decides the
	// result, otherwise these guards would report a division by zero.
	assertInspect(t, eval(t, "false && 1 / 0 == 0"), "false")
	assertInspect(t, eval(t, "true || 1 / 0 == 0"), "true")
	assertInspect(t, eval(t, "let xs = [1]; let i = 5; i < len(xs) && xs[i] != null"), "false")

	// A failing right operand is still an error when it is reached.
	assertError(t, eval(t, "true && 1 / 0 == 0"), "division by zero")
	assertError(t, eval(t, "false || 1 / 0 == 0"), "division by zero")
}

// -------------------------------------------------------------- conditionals

func TestIfElseExpressions(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{"if true { 10 }", int64(10)},
		{"if 1 < 2 { 10 }", int64(10)},
		{"if false { 10 }", nil},
		{"if 0 { 10 }", nil},
		{"if 1 > 2 { 10 } else { 20 }", int64(20)},
		{"if 1 < 2 { 10 } else { 20 }", int64(10)},
		{"if 1 < 2 { if 1 < 2 { 1 } else { 2 } }", int64(1)},
		{"if 1 > 2 { 1 } else if 1 < 2 { 2 } else { 3 }", int64(2)},
		{"if 1 > 2 { 1 } else if 1 > 2 { 2 } else { 3 }", int64(3)},
		{"if 1 > 2 { 1 } else if 1 > 2 { 2 } else if 1 < 2 { 4 } else { 3 }", int64(4)},
	}

	for _, tt := range tests {
		obj := eval(t, tt.input)

		if tt.want == nil {
			if obj.Type() != object.NULL {
				t.Errorf("input %q: expected NULL, got %s (%s)", tt.input, obj.Type(), obj.Inspect())
			}
			continue
		}

		want, ok := tt.want.(int64)
		if !ok {
			t.Fatalf("bad test case for %q", tt.input)
		}

		got, ok := obj.(*object.Integer)
		if !ok {
			t.Fatalf("input %q: expected *object.Integer, got %T (%s)", tt.input, obj, obj.Inspect())
		}
		if got.Value != want {
			t.Errorf("input %q: expected %d, got %d", tt.input, want, got.Value)
		}
	}
}

func TestIfConditionTypeErrors(t *testing.T) {
	// A condition that is not comparable is an error even when the branch
	// would not be taken.
	assertError(t, eval(t, "if 5 + true { 10 }"), "type mismatch: INTEGER + BOOLEAN")
}

// -------------------------------------------------------------------- strings

func TestStringOperations(t *testing.T) {
	assertInspect(t, eval(t, `"hello" + " " + "world"`), "hello world")
	assertInspect(t, eval(t, `"a" == "a"`), "true")
	assertInspect(t, eval(t, `"a" == "b"`), "false")
	assertInspect(t, eval(t, `"a" != "b"`), "true")
	assertInspect(t, eval(t, `"a" < "b"`), "true")
	assertInspect(t, eval(t, `"b" < "a"`), "false")
	assertInspect(t, eval(t, `len("hello")`), "5")
}

func TestStringErrors(t *testing.T) {
	assertError(t, eval(t, `"a" - "b"`), "unknown operator: STRING - STRING")
	assertError(t, eval(t, `"a" * 2`), "type mismatch: STRING * INTEGER")
	assertError(t, eval(t, `5 + "a"`), "type mismatch: INTEGER + STRING")
	assertError(t, eval(t, `"a" / 2`), "type mismatch: STRING / INTEGER")
}

func TestCrossTypeEqualityIsFalseNotAnError(t *testing.T) {
	assertInspect(t, eval(t, `1 == "1"`), "false")
	assertInspect(t, eval(t, `1 != "1"`), "true")
	assertInspect(t, eval(t, `1 == null`), "false")
	assertInspect(t, eval(t, `null == null`), "true")
	assertInspect(t, eval(t, `true == 1`), "false")
}

func TestOrderingAcrossTypesIsAnError(t *testing.T) {
	assertError(t, eval(t, `1 < "a"`), "type mismatch: INTEGER < STRING")
}

// ------------------------------------------------------------- let and scope

func TestLetStatements(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"let a = 5; a;", 5},
		{"let a = 5 * 5; a;", 25},
		{"let a = 5; let b = a; b;", 5},
		{"let a = 5; let b = a; let c = a + b + 5; c;", 15},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestLetIsBlockScoped(t *testing.T) {
	assertInteger(t, eval(t, "let a = 1; if true { let a = 2; } a;"), 1)
	assertInteger(t, eval(t, "let a = 1; { let a = 2; } a;"), 1)
	assertInteger(t, eval(t, "let a = 1; { let a = 2; a; }"), 2)
}

func TestShadowedVariableIsUndefinedOutsideTheBlock(t *testing.T) {
	assertError(t, eval(t, "if true { let b = 2; } b;"), "identifier not found: b")
}

func TestAssignment(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"let a = 1; a = 2; a;", 2},
		{"let a = 1; a = a + 1; a;", 2},
		// Assignment must reach out to the enclosing scope rather than
		// shadowing, otherwise loops can never make progress.
		{"let a = 1; if true { a = 2; } a;", 2},
		{"let a = 1; while a < 5 { a = a + 1; } a;", 5},
		{"let a = 1; let f = fn() { a = 99; }; f(); a;", 99},
		{"a = 7; a;", 7},
		{"let a = 1; let b = 2; a = b = 3; a + b;", 6},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestAssignToArrayElement(t *testing.T) {
	assertInspect(t, eval(t, "let xs = [1, 2, 3]; xs[0] = 99; xs;"), "[99, 2, 3]")
	assertInspect(t, eval(t, "let xs = [[1], [2]]; xs[1][0] = 42; xs;"), "[[1], [42]]")
}

func TestAssignmentExpressionEvaluatesToTheAssignedValue(t *testing.T) {
	assertInteger(t, eval(t, "let a = 1; a = 5;"), 5)
}

// --------------------------------------------------------------------- return

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"return 10;", 10},
		{"return 10; 9;", 10},
		{"return 2 * 5; 9;", 10},
		{"9; return 2 * 5; 9;", 10},
		{"if 10 > 1 { if 10 > 1 { return 10; } return 1; }", 10},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestReturnUnwindsBlocksLoopsAndFunctions(t *testing.T) {
	assertInteger(t, eval(t, "fn f() { { return 1; } 2; } f();"), 1)
	assertInteger(t, eval(t, "fn f() { while true { return 3; } } f();"), 3)
	assertInteger(t, eval(t, "fn f() { for let i = 0; i < 10; i = i + 1 { if i == 4 { return i; } } -1; } f();"), 4)
	assertInteger(t, eval(t, "fn f() { let i = 0; while i < 10 { i = i + 1; if i == 5 { return i; } } -1; } f();"), 5)
}

func TestImplicitReturnOfLastExpression(t *testing.T) {
	assertInteger(t, eval(t, "fn f() { 42 } f();"), 42)
	assertInteger(t, eval(t, "fn f() { let a = 1; a + 1 } f();"), 2)
	assertInspect(t, eval(t, "fn f() { } f();"), "null")
}

// ------------------------------------------------------------------- functions

func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"let identity = fn(x) { x; }; identity(5);", 5},
		{"let identity = fn(x) { return x; }; identity(5);", 5},
		{"let double = fn(x) { x * 2; }; double(5);", 10},
		{"let add = fn(x, y) { x + y; }; add(5, 5);", 10},
		{"let add = fn(x, y) { x + y; }; add(5 + 5, add(5, 5));", 20},
		{"fn(x) { x; }(5)", 5},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestFunctionDeclaration(t *testing.T) {
	assertInteger(t, eval(t, "fn add(a, b) { return a + b; } add(1, 2);"), 3)
	assertInteger(t, eval(t, "fn pi() { return 3; } pi();"), 3)
}

func TestRecursion(t *testing.T) {
	assertInteger(t, eval(t, "fn fact(n) { if n <= 1 { return 1; } return n * fact(n - 1); } fact(5);"), 120)
	assertInteger(t, eval(t, "let fib = fn(n) { if n < 2 { return n; } return fib(n-1) + fib(n-2); }; fib(10);"), 55)
	assertInteger(t, eval(t, "fn even(n) { if n == 0 { return true; } return odd(n - 1); } fn odd(n) { if n == 0 { return false; } return even(n - 1); } if even(10) { 1 } else { 0 };"), 1)
}

func TestClosures(t *testing.T) {
	assertInteger(t, eval(t, "let newAdder = fn(x) { fn(y) { x + y; }; }; let addTwo = newAdder(2); addTwo(2);"), 4)

	// A counter closure must keep state across calls.
	counter := `
fn makeCounter() {
	let count = 0;
	return fn() { count = count + 1; return count; };
}
let c = makeCounter();
c(); c(); c();
`
	assertInteger(t, eval(t, counter), 3)

	// Two counters must not share state.
	twoCounters := `
fn makeCounter() {
	let count = 0;
	return fn() { count = count + 1; return count; };
}
let a = makeCounter();
let b = makeCounter();
a(); a();
b();
`
	assertInteger(t, eval(t, twoCounters), 1)
}

func TestFunctionsAreHigherOrderValues(t *testing.T) {
	assertInteger(t, eval(t, "let apply = fn(f, x) { return f(x); }; apply(fn(x) { x * 2; }, 21);"), 42)
	assertInspect(t, eval(t, "fn(x) { x; }"), "fn(x) { x; }")
	assertInteger(t, eval(t, "let fs = [fn() { 1; }, fn() { 2; }]; fs[1]();"), 2)
}

func TestWrongArgumentCount(t *testing.T) {
	assertError(t, eval(t, "fn(a, b) { a + b; }(1);"), "wrong number of arguments: want 2, got 1")
	assertError(t, eval(t, "fn(a) { a; }(1, 2);"), "wrong number of arguments: want 1, got 2")
	assertError(t, eval(t, "fn() { 1; }(1);"), "wrong number of arguments: want 0, got 1")
	assertError(t, eval(t, "fn(x) { x; }()"), "wrong number of arguments: want 1, got 0")
}

func TestCallingANonFunction(t *testing.T) {
	assertError(t, eval(t, "5()"), "not a function: INTEGER")
	assertError(t, eval(t, "let x = 5; x();"), "not a function: INTEGER")
	assertError(t, eval(t, `"abc"()`), "not a function: STRING")
}

func TestFunctionParameterShadowsOuterBinding(t *testing.T) {
	assertInteger(t, eval(t, "let x = 1; let f = fn(x) { x * 2; }; f(10); x;"), 1)
}

// ---------------------------------------------------------------------- loops

func TestWhileLoop(t *testing.T) {
	assertInteger(t, eval(t, "let i = 0; while i < 5 { i = i + 1; } i;"), 5)
	assertInteger(t, eval(t, "let i = 0; let sum = 0; while i < 10 { sum = sum + i; i = i + 1; } sum;"), 45)
	assertInteger(t, eval(t, "while false { 1; } 7;"), 7)
}

func TestForLoop(t *testing.T) {
	assertInteger(t, eval(t, "let sum = 0; for let i = 0; i < 10; i = i + 1 { sum = sum + i; } sum;"), 45)
	assertInteger(t, eval(t, "let n = 0; for ; n < 3; { n = n + 1; } n;"), 3)
	assertInteger(t, eval(t, "let i = 0; for ; i < 3; i = i + 1 { } i;"), 3)
	assertInteger(t, eval(t, "let sum = 0; for let i = 0; i < 3; i = i + 1 { for let j = 0; j < 3; j = j + 1 { sum = sum + 1; } } sum;"), 9)
}

func TestLoopBodiesGetAFreshScopeEachIteration(t *testing.T) {
	// A `let` inside the body is re-created every iteration, so it does not
	// accumulate.
	assertInteger(t, eval(t, "let count = 0; for let i = 0; i < 3; i = i + 1 { let x = i; count = count + x; } count;"), 3)
}

func TestLoopOutput(t *testing.T) {
	got := evalOutput(t, "for let i = 0; i < 3; i = i + 1 { print(i); }")
	if got != "0\n1\n2\n" {
		t.Errorf("expected \"0\\n1\\n2\\n\", got %q", got)
	}

	got = evalOutput(t, "let i = 0; while i < 2 { print(i); i = i + 1; }")
	if got != "0\n1\n" {
		t.Errorf("expected \"0\\n1\\n\", got %q", got)
	}
}

func TestInfiniteLoopIsStoppedByTheStepLimit(t *testing.T) {
	ev := New(io.Discard)
	ev.MaxSteps = 1000

	obj := evalWith(t, "while true { }", ev)
	assertError(t, obj, "step limit")
}

func TestStepLimitAlsoCatchesRunawayRecursion(t *testing.T) {
	ev := New(io.Discard)
	ev.MaxSteps = 10000

	obj := evalWith(t, "fn f(n) { return f(n + 1); } f(0);", ev)
	assertError(t, obj, "step limit")
}

func TestDeepButFiniteRecursionIsAllowed(t *testing.T) {
	assertInteger(t, eval(t, "fn count(n) { if n == 0 { return 0; } return count(n - 1); } count(2000);"), 0)
}

// --------------------------------------------------------------------- arrays

func TestArrayLiterals(t *testing.T) {
	assertInspect(t, eval(t, "[1, 2 * 2, 3 + 3]"), "[1, 4, 6]")
	assertInspect(t, eval(t, "[]"), "[]")
	assertInspect(t, eval(t, `["a", "b"]`), "[a, b]")
	assertInspect(t, eval(t, "[1, [2, 3]]"), "[1, [2, 3]]")
}

func TestIndexExpressions(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"[1, 2, 3][0]", 1},
		{"[1, 2, 3][1]", 2},
		{"[1, 2, 3][2]", 3},
		{"let i = 0; [1][i];", 1},
		{"[1, 2, 3][1 + 1]", 3},
		{"let myArray = [1, 2, 3]; myArray[2];", 3},
		{"let myArray = [1, 2, 3]; myArray[0] + myArray[1] + myArray[2];", 6},
		{"let myArray = [1, 2, 3]; let i = myArray[0]; myArray[i];", 2},
		{"[[1, 2]][0][1]", 2},
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestIndexReadsOutOfRangeYieldNull(t *testing.T) {
	// Reading outside an array is not a crash and not an error: it yields
	// null, which callers can test with `== null`.
	assertInspect(t, eval(t, "[1, 2, 3][3]"), "null")
	assertInspect(t, eval(t, "[1, 2, 3][99]"), "null")
	assertInspect(t, eval(t, "[1, 2, 3][-1]"), "null")
	assertInspect(t, eval(t, "[][0]"), "null")
}

func TestIndexErrors(t *testing.T) {
	assertError(t, eval(t, `[1, 2, 3]["a"]`), "index must be an integer, got STRING")
	assertError(t, eval(t, "5[0]"), "index operator not supported: INTEGER")
	assertError(t, eval(t, `"abc"[0]`), "index operator not supported: STRING")
	assertError(t, eval(t, "let xs = [1]; xs[5] = 1;"), "index out of range: 5 (array length 1)")
	assertError(t, eval(t, "let xs = [1]; xs[-1] = 1;"), "index out of range: -1 (array length 1)")
}

// -------------------------------------------------------------------- builtins

func TestBuiltinLen(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len("hello world")`, 11},
		{"len([])", 0},
		{"len([1, 2, 3])", 3},
		{`len("héllo")`, 5}, // counts runes, not bytes
	}

	for _, tt := range tests {
		assertInteger(t, eval(t, tt.input), tt.want)
	}
}

func TestBuiltinLenErrors(t *testing.T) {
	assertError(t, eval(t, "len(1)"), "argument to `len` not supported, got INTEGER")
	assertError(t, eval(t, `len("one", "two")`), "wrong number of arguments to `len`: want 1, got 2")
	assertError(t, eval(t, "len()"), "wrong number of arguments to `len`: want 1, got 0")
}

func TestBuiltinPush(t *testing.T) {
	assertInspect(t, eval(t, "push([1, 2], 3)"), "[1, 2, 3]")
	assertInspect(t, eval(t, "push([], 1)"), "[1]")
	assertInspect(t, eval(t, "let xs = [1]; push(xs, 2); xs;"), "[1]") // push copies
	assertInspect(t, eval(t, `push(["a"], "b")`), "[a, b]")
	assertError(t, eval(t, "push(1, 2)"), "argument to `push` must be an array, got INTEGER")
	assertError(t, eval(t, "push([1])"), "wrong number of arguments to `push`: want 2, got 1")
}

func TestBuiltinFirstLastRest(t *testing.T) {
	assertInteger(t, eval(t, "first([1, 2, 3])"), 1)
	assertInteger(t, eval(t, "last([1, 2, 3])"), 3)
	assertInspect(t, eval(t, "rest([1, 2, 3])"), "[2, 3]")
	assertInspect(t, eval(t, "first([])"), "null")
	assertInspect(t, eval(t, "last([])"), "null")
	assertInspect(t, eval(t, "rest([])"), "null")
}

func TestBuiltinType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`type(1)`, "INTEGER"},
		{`type(1.5)`, "FLOAT"},
		{`type("a")`, "STRING"},
		{`type(true)`, "BOOLEAN"},
		{`type(null)`, "NULL"},
		{`type([])`, "ARRAY"},
		{`type(fn(){})`, "FUNCTION"},
		{`type(len)`, "BUILTIN"},
	}

	for _, tt := range tests {
		assertInspect(t, eval(t, tt.input), tt.want)
	}
}

func TestPrintBuiltinFormatsValues(t *testing.T) {
	got := evalOutput(t, `
print("hello");
print(42);
print(1.5);
print(true);
print(null);
print([1, "two", [3]]);
print();
`)
	want := "hello\n42\n1.5\ntrue\nnull\n[1, two, [3]]\n\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuiltinsAreFirstClassValues(t *testing.T) {
	// `len` is a value, so it can be bound to a name and called indirectly.
	assertInteger(t, eval(t, "let f = len; f(\"abc\");"), 3)
	assertError(t, eval(t, "let f = len; f();"), "wrong number of arguments to `len`: want 1, got 0")
}

func TestStringBuiltin(t *testing.T) {
	// str() renders any value as a string, which makes output easy.
	assertInspect(t, eval(t, `str(42)`), "42")
	assertInspect(t, eval(t, `str(1.5)`), "1.5")
	assertInspect(t, eval(t, `str(true)`), "true")
	assertInspect(t, eval(t, `str([1, 2])`), "[1, 2]")
	assertInspect(t, eval(t, `str("a") + str("b")`), "ab")
}

// --------------------------------------------------------------------- errors

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"5 + true;", "type mismatch: INTEGER + BOOLEAN"},
		{"5 + true; 5;", "type mismatch: INTEGER + BOOLEAN"},
		{"-true", "unknown operator: -BOOLEAN"},
		{"true + false;", "unknown operator: BOOLEAN + BOOLEAN"},
		{"5; true + false; 5", "unknown operator: BOOLEAN + BOOLEAN"},
		{"if 10 > 1 { true + false; }", "unknown operator: BOOLEAN + BOOLEAN"},
		{"if 10 > 1 { if 10 > 1 { return true + false; } return 1; }", "unknown operator: BOOLEAN + BOOLEAN"},
		{"foobar", "identifier not found: foobar"},
		{"let x = 1; foobar;", "identifier not found: foobar"},
		{`"Hello" - "World"`, "unknown operator: STRING - STRING"},
		{"1 / 0", "division by zero"},
	}

	for _, tt := range tests {
		assertError(t, eval(t, tt.input), tt.want)
	}
}

func TestErrorsPropagateOutOfLoopsAndFunctions(t *testing.T) {
	assertError(t, eval(t, "while true { 5 + true; }"), "type mismatch")
	assertError(t, eval(t, "for let i = 0; i < 3; i = i + 1 { 5 + true; }"), "type mismatch")
	assertError(t, eval(t, "fn f() { 5 + true; } f();"), "type mismatch")
	assertError(t, eval(t, "fn f() { return undefinedThing; } f();"), "identifier not found")
}

func TestErrorsCarryAPosition(t *testing.T) {
	p := parser.New(lexer.New("let a = 1;\n5 + true;"))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parser errors: %v", errs)
	}

	obj := New(io.Discard).Eval(program, object.NewEnvironment())
	errObj, ok := obj.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T", obj)
	}
	if errObj.Pos.Line != 2 {
		t.Errorf("expected error on line 2, got %s", errObj.Pos)
	}
}

func TestErrorsStopEvaluation(t *testing.T) {
	// Once an error is produced it must not be swallowed by a later
	// statement, otherwise programs fail silently.
	got := evalOutput(t, "5 + true; print(\"should not print\");")
	if got != "" {
		t.Errorf("expected no output after an error, got %q", got)
	}
}

func TestInspectOfEveryObjectType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"5", "5"},
		{"-5", "-5"},
		{"1.5", "1.5"},
		{"true", "true"},
		{"false", "false"},
		{"null", "null"},
		{`"hi"`, "hi"},
		{"[1, 2]", "[1, 2]"},
		{"fn(x) { x; }", "fn(x) { x; }"},
		{"len", "builtin function: len"},
	}

	for _, tt := range tests {
		assertInspect(t, eval(t, tt.input), tt.want)
	}
}

// TestFloatOutputUsesPlainDecimalNotation checks what a program actually
// writes for a float.
//
// `print` and `str` are how a program shows a number to whoever is reading,
// and a notation the language itself cannot parse is no use to them: the
// output of `print(1000000.0)` used to be "1e+06". Whole numbers are written
// without a fractional part, which is the convention examples/lang.cw
// documents at its `print(1.5 + 2.5)` line.
func TestFloatOutputUsesPlainDecimalNotation(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"the value that first exposed it", "print(1000000.0);", "1000000\n"},
		{"negative", "print(-1000000.0);", "-1000000\n"},
		{"a whole number keeps no fractional part", "print(1.5 + 2.5);", "4\n"},
		{"a fraction is unchanged", "print(3.5);", "3.5\n"},
		{"less than one", "print(0.125);", "0.125\n"},
		{"small enough to have reached for an exponent", "print(0.0000001);", "0.0000001\n"},
		{"large enough to have reached for an exponent", "print(1000000000000000000000.0);", "1000000000000000000000\n"},
		{"a third still rounds to the shortest form", "print(1.0 / 3.0);", "0.3333333333333333\n"},
		{"str uses the same form", "print(str(1000000.0));", "1000000\n"},
		{"so does a float inside an array", "print([1.0, 1000000.0]);", "[1, 1000000]\n"},
		{"and a float read back out of one", "let xs = [1000000.0]; print(xs[0]);", "1000000\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalOutput(t, tt.src); got != tt.want {
				t.Errorf("%s printed %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

// TestDocumentedBehaviour covers claims made in docs/LANGUAGE.md that the
// rest of the suite does not otherwise pin down.
func TestIfIsAnExpression(t *testing.T) {
	assertInspect(t, eval(t, `let label = if 1 < 0 { "negative" } else { "non-negative" }; label;`), "non-negative")
	assertInspect(t, eval(t, `print(if true { "yes" } else { "no" });`), "null")
	assertInspect(t, eval(t, `[if true { 1 } else { 2 }, if false { 1 } else { 2 }]`), "[1, 2]")
	assertInspect(t, eval(t, `fn pick(b) { if b { "on" } else { "off" }; } pick(true);`), "on")
}

func TestPrintAcceptsSeveralArguments(t *testing.T) {
	got := evalOutput(t, `print("a", 1, true, [1, 2]);`)
	want := "a 1 true [1, 2]\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	got = evalOutput(t, `print();`)
	if got != "\n" {
		t.Errorf("expected a bare newline, got %q", got)
	}
}

func TestIntegerAndFloatCompareEqualByValue(t *testing.T) {
	assertInspect(t, eval(t, "1 == 1.0"), "true")
	assertInspect(t, eval(t, "1 != 1.0"), "false")
	assertInspect(t, eval(t, "2 == 2.5"), "false")
}

func TestStringOrderingOperators(t *testing.T) {
	assertInspect(t, eval(t, `"a" <= "a"`), "true")
	assertInspect(t, eval(t, `"a" >= "b"`), "false")
	assertInspect(t, eval(t, `"abc" > "abd"`), "false")
}

func TestForLoopWithNoConditionLoopsUntilItReturns(t *testing.T) {
	assertInteger(t, eval(t, "fn f() { let i = 0; for ; ; i = i + 1 { if i == 5 { return i; } } } f();"), 5)
}

func TestTypeOfAClosureIsFunction(t *testing.T) {
	assertInspect(t, eval(t, "fn make() { let n = 1; return fn() { n; }; } type(make());"), "FUNCTION")
}

func TestNestedArraysAndDeepIndexing(t *testing.T) {
	assertInspect(t, eval(t, "let grid = [[1, 2], [3, 4]]; grid[1][0];"), "3")
	assertInspect(t, eval(t, "let grid = [[1, 2], [3, 4]]; grid[0][1] = 99; grid;"), "[[1, 99], [3, 4]]")
}

func TestShadowingInNestedFunctions(t *testing.T) {
	// The innermost binding wins, and the outer ones are left alone.
	assertInteger(t, eval(t, "let x = 1; fn f() { let x = 2; fn g() { let x = 3; return x; } g(); return x; } f() + x;"), 3)
}

func TestBuiltinErrorsNameTheBuiltin(t *testing.T) {
	assertError(t, eval(t, "first(1)"), "argument to `first` must be an array, got INTEGER")
	assertError(t, eval(t, "last(1)"), "argument to `last` must be an array, got INTEGER")
	assertError(t, eval(t, "rest(1)"), "argument to `rest` must be an array, got INTEGER")
	assertError(t, eval(t, "type()"), "wrong number of arguments to `type`: want 1, got 0")
	assertError(t, eval(t, "str()"), "wrong number of arguments to `str`: want 1, got 0")
}

// --------------------------------------------------------------- integration

func TestRealisticPrograms(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "fizzbuzz",
			input: `
for let i = 1; i <= 15; i = i + 1 {
	if i % 15 == 0 {
		print("FizzBuzz");
	} else if i % 3 == 0 {
		print("Fizz");
	} else if i % 5 == 0 {
		print("Buzz");
	} else {
		print(i);
	}
}
`,
			want: "1\n2\nFizz\n4\nBuzz\nFizz\n7\n8\nFizz\nBuzz\n11\nFizz\n13\n14\nFizzBuzz\n",
		},
		{
			name: "fibonacci with recursion",
			input: `
fn fib(n) {
	if n < 2 { return n; }
	return fib(n - 1) + fib(n - 2);
}
for let i = 0; i < 10; i = i + 1 { print(fib(i)); }
`,
			want: "0\n1\n1\n2\n3\n5\n8\n13\n21\n34\n",
		},
		{
			name: "map and reduce over arrays",
			input: `
fn map(xs, f) {
	let out = [];
	let i = 0;
	while i < len(xs) { out = push(out, f(xs[i])); i = i + 1; }
	return out;
}
fn reduce(xs, f, acc) {
	let i = 0;
	while i < len(xs) { acc = f(acc, xs[i]); i = i + 1; }
	return acc;
}
let numbers = [1, 2, 3, 4, 5];
let doubled = map(numbers, fn(x) { x * 2; });
print(doubled);
print(reduce(doubled, fn(a, b) { a + b; }, 0));
print(reduce(numbers, fn(a, b) { a * b; }, 1));
`,
			want: "[2, 4, 6, 8, 10]\n30\n120\n",
		},
		{
			name: "counter closure",
			input: `
fn makeCounter(start) {
	let n = start;
	return fn() { n = n + 1; return n; };
}
let next = makeCounter(10);
print(next());
print(next());
print(next());
`,
			want: "11\n12\n13\n",
		},
		{
			name: "gcd and string building",
			input: `
fn gcd(a, b) {
	while b != 0 {
		let t = b;
		b = a % b;
		a = t;
	}
	return a;
}
print(gcd(48, 18));
print("result: " + "done");
`,
			want: "6\nresult: done\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalOutput(t, tt.input)
			if got != tt.want {
				t.Errorf("expected output:\n%q\ngot:\n%q", tt.want, got)
			}
		})
	}
}

// ------------------------------------------------------- recursion limits
//
// A tree-walking evaluator recurses once per nested call, so the depth a
// program can reach is bounded by the Go stack. Go cannot recover from
// exhausting it, so the evaluator has to refuse to descend past a documented
// limit and say so, rather than finding the limit by dying.

// TestDeepButLegitimateRecursionIsAllowed guards the call depth limit against
// being set so low that ordinary recursive programs stop working.
func TestDeepButLegitimateRecursionIsAllowed(t *testing.T) {
	const depth = 1000

	src := fmt.Sprintf(`
fn countdown(n) {
    if n <= 0 { return 0; }
    return 1 + countdown(n - 1);
}
countdown(%d);
`, depth)

	assertInteger(t, eval(t, src), depth)
}

// TestRecursionBeyondTheLimitIsRejected keeps a program that recurses past the
// limit from taking the process down with it. The limit is generous, so a
// program that reaches it is almost certainly a bug in the program.
func TestRecursionBeyondTheLimitIsRejected(t *testing.T) {
	const depth = 50_000

	src := fmt.Sprintf(`
fn countdown(n) {
    if n <= 0 { return 0; }
    return 1 + countdown(n - 1);
}
countdown(%d);
`, depth)

	assertError(t, eval(t, src), "call depth")
}

// TestRunawayRecursionDoesNotCrashTheProcess is the regression guard for the
// most serious failure this evaluator had: `fn f(n) { return f(n + 1); }`
// exhausted the stack and aborted the process with a Go runtime dump. The step
// limit does not help here, because the stack dies long before the step
// counter reaches its budget.
func TestRunawayRecursionDoesNotCrashTheProcess(t *testing.T) {
	assertError(t, eval(t, `
fn f(n) { return f(n + 1); }
f(0);
`), "call depth")

	// Mutual recursion must be caught too: each function on its own looks
	// innocent, and only the pair is runaway.
	assertError(t, eval(t, `
fn ping(n) { return pong(n + 1); }
fn pong(n) { return ping(n + 1); }
ping(0);
`), "call depth")

	// The same, reached through an anonymous function.
	assertError(t, eval(t, `
let f = fn(n) { return f(n + 1); };
f(0);
`), "call depth")
}

// TestCallDepthLimitIsConfigurable keeps the limit a policy rather than a
// hard-wired constraint, the way MaxSteps already is.
func TestCallDepthLimitIsConfigurable(t *testing.T) {
	const depth = 50_000

	src := fmt.Sprintf(`
fn countdown(n) {
    if n <= 0 { return 0; }
    return 1 + countdown(n - 1);
}
countdown(%d);
`, depth)

	ev := New(io.Discard)
	ev.MaxDepth = depth * 2

	assertInteger(t, evalWith(t, src, ev), depth)
}

// TestRecursionDepthDoesNotLeakBetweenCalls checks that the depth counter is
// unwound on the way out. A counter that only ever grows would make a long
// REPL session fail once its cumulative calls passed the limit.
func TestRecursionDepthDoesNotLeakBetweenCalls(t *testing.T) {
	ev := New(io.Discard)
	ev.MaxDepth = 50

	src := `
fn countdown(n) {
    if n <= 0 { return 0; }
    return 1 + countdown(n - 1);
}
countdown(40);
`

	// 40 deep, well inside the limit of 50, but enough repetitions would
	// exceed it if the counter were never reset.
	for i := 0; i < 20; i++ {
		assertInteger(t, evalWith(t, src, ev), 40)
	}
}

// TestLoopsThatEvaluateNothingStillHonourTheStepLimit is the regression guard
// for a `for` loop the step limit could not see. `for ;; { }` has no condition
// to evaluate and a body that evaluates nothing, so a limit that only counted
// visited AST nodes never advanced and the loop ran forever: the process had
// to be killed. Every loop iteration is now counted, whatever its body does.
func TestLoopsThatEvaluateNothingStillHonourTheStepLimit(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"for with no condition and an empty body", "for ;; { }"},
		{"for with no clauses and an empty body", "for ; ; { }"},
		{"for with an init but no condition", "for let i = 0; ; { }"},
		{"for with a post but no condition", "let i = 0; for ; ; i = i + 1 { }"},
		{"while true with an empty body", "while true { }"},
		{"nested empty loops", "for ;; { for ;; { } }"},
		{"for whose body is only declarations", "for ;; { let x = 1; }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := New(io.Discard)
			ev.MaxSteps = 1000

			got := evalWith(t, tt.src, ev)
			assertError(t, got, "step limit")
		})
	}
}

// TestStepLimitIsNotSpentByOrdinaryLoops keeps the per-iteration step from
// making the budget so expensive that reasonable programs run out of it.
func TestStepLimitIsNotSpentByOrdinaryLoops(t *testing.T) {
	ev := New(io.Discard)
	ev.MaxSteps = 100_000

	assertInteger(t, evalWith(t, `
let total = 0;
for let i = 0; i < 500; i = i + 1 {
    total = total + i;
}
total;
`, ev), 124750)
}

// TestManyToplevelBindingsStayVisible is the regression guard for a program
// that lost every binding past the ninth.
//
// A scope kept its names in a slice and built a name index once it grew past a
// threshold. Adding a further name appended to the slice without updating the
// index, and lookups consult the index once it exists, so the tenth top-level
// `let` in a program was written and then reported as undefined. Nine bindings
// worked and ten did not, which is why a wide program — not a deep one — was
// what exposed it.
func TestManyToplevelBindingsStayVisible(t *testing.T) {
	const count = 50

	var src strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&src, "let v%d = %d;\n", i, i)
	}
	// Read them back in reverse, so that a binding is not found merely because
	// something next to it was.
	for i := count - 1; i >= 0; i-- {
		fmt.Fprintf(&src, "print(v%d);\n", i)
	}

	var want strings.Builder
	for i := count - 1; i >= 0; i-- {
		fmt.Fprintf(&want, "%d\n", i)
	}

	if got := evalOutput(t, src.String()); got != want.String() {
		t.Errorf("wrong output for %d top-level bindings:\n--- want ---\n%s--- got ---\n%s",
			count, want.String(), got)
	}
}

// TestAssignmentReachesAWideToplevelScope checks that a block can assign to one
// of many top-level variables.
//
// The same lost binding made assignment fall back to declaring the name in the
// block, so the assignment appeared to succeed while the variable outside kept
// its old value — a wrong answer rather than an error.
func TestAssignmentReachesAWideToplevelScope(t *testing.T) {
	const count = 20

	var src strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&src, "let v%d = %d;\n", i, i)
	}
	// Assign to the last one from inside a block, then read it outside.
	fmt.Fprintf(&src, "{ v%d = 99; }\nprint(v%d);\n", count-1, count-1)

	if got := evalOutput(t, src.String()); got != "99\n" {
		t.Errorf("assignment did not reach the top-level binding: got %q, want %q", got, "99\n")
	}
}

// TestClosureCapturesAWideScope checks that a function defined next to many
// bindings can still see all of them.
func TestClosureCapturesAWideScope(t *testing.T) {
	const count = 30

	var src strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&src, "let v%d = %d;\n", i, i)
	}
	src.WriteString("fn last() { return v29; }\nprint(last());\n")

	if got := evalOutput(t, src.String()); got != "29\n" {
		t.Errorf("closure could not see a late binding: got %q, want %q", got, "29\n")
	}
}

// TestEqualityIsTotalSymmetricAndReflexive checks the three properties the
// language promises about `==`, over every ordered pair of values rather than
// the handful of pairs that happened to get written down.
//
// They are: that comparing any two values of any two types yields a boolean
// and never an error; that `a == b` and `b == a` agree; and that `!=` is its
// negation. Equality is the one operator defined between every pair of types,
// which is what makes `x == null` safe to write, so it is worth pinning down
// across the whole cross product instead of case by case.
func TestEqualityIsTotalSymmetricAndReflexive(t *testing.T) {
	values := []struct {
		name string
		expr string
	}{
		{"integer", "1"},
		{"another integer", "2"},
		{"float", "1.0"},
		{"another float", "2.5"},
		{"a float that is a whole number", "2.0"},
		{"string", `"1"`},
		{"another string", `"abc"`},
		{"empty string", `""`},
		{"true", "true"},
		{"false", "false"},
		{"null", "null"},
		{"array", "[1, 2]"},
		{"empty array", "[]"},
		{"nested array", "[[1], [2]]"},
		{"function", "fn(x) { x; }"},
		{"builtin", "len"},
	}

	// evalBool evaluates an expression that must produce a boolean. Anything
	// else — including an error value — fails the test.
	evalBool := func(t *testing.T, name, src string) bool {
		t.Helper()

		obj := eval(t, src)
		b, ok := obj.(*object.Boolean)
		if !ok {
			t.Fatalf("%s: expected a boolean, got %T (%s)", name, obj, obj.Inspect())
		}
		return b.Value
	}

	for _, a := range values {
		// Reflexivity: a value equals itself, whatever it is.
		t.Run("reflexive/"+a.name, func(t *testing.T) {
			if !evalBool(t, a.name, "let x = "+a.expr+"; x == x;") {
				t.Errorf("%s is not equal to itself", a.name)
			}
			if evalBool(t, a.name, "let x = "+a.expr+"; x != x;") {
				t.Errorf("%s is not equal to itself, but != says it is not", a.name)
			}
		})

		for _, b := range values {
			t.Run("pair/"+a.name+"/"+b.name, func(t *testing.T) {
				forward := evalBool(t, "a == b",
					"let a = "+a.expr+"; let b = "+b.expr+"; a == b;")
				backward := evalBool(t, "b == a",
					"let a = "+a.expr+"; let b = "+b.expr+"; b == a;")
				unequal := evalBool(t, "a != b",
					"let a = "+a.expr+"; let b = "+b.expr+"; a != b;")

				if forward != backward {
					t.Errorf("%s == %s is %v but %s == %s is %v",
						a.name, b.name, forward, b.name, a.name, backward)
				}
				if unequal == forward {
					t.Errorf("%s != %s is %v, which is not the negation of == (%v)",
						a.name, b.name, unequal, forward)
				}
			})
		}
	}

	// The pairs the documentation makes a specific promise about.
	t.Run("values the docs pin down", func(t *testing.T) {
		yes := []struct {
			name string
			src  string
		}{
			{"an integer and a float of the same value", "1 == 1.0;"},
			{"a whole float and an integer", "2.0 == 2;"},
			{"null and null", "null == null;"},
			{"two equal strings", `"ab" == "ab";`},
			{"two equal arrays", "let xs = [1, 2]; xs == xs;"},
		}
		for _, tt := range yes {
			if !evalBool(t, tt.name, tt.src) {
				t.Errorf("%s: %s is false, want true", tt.name, tt.src)
			}
		}

		no := []struct {
			name string
			src  string
		}{
			{"an integer and a string", `1 == "1";`},
			{"an integer and null", "1 == null;"},
			{"a boolean and an integer", "true == 1;"},
			{"null and false", "null == false;"},
			{"an array and a string", `[1] == "1";`},
			{"different integers", "1 == 2;"},
			{"an integer and an unequal float", "1 == 1.5;"},
			{"two separately built but equal arrays", "[1, 2] == [1, 2];"},
		}
		for _, tt := range no {
			if evalBool(t, tt.name, tt.src) {
				t.Errorf("%s: %s is true, want false", tt.name, tt.src)
			}
		}
	})
}
