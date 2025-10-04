package parser

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
)

// parse is a helper that parses input and fails the test on any parser error.
func parse(t *testing.T, input string) *ast.Program {
	t.Helper()

	l := lexer.New(input)
	if errs := l.Errors(); len(errs) != 0 {
		t.Fatalf("unexpected lexer errors: %v", errs)
	}

	p := New(l)
	program := p.ParseProgram()

	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parser errors for input %q:\n  %v", input, errs)
	}

	return program
}

// parseWithErrors parses input and returns the parser's error strings.
func parseWithErrors(t *testing.T, input string) []string {
	t.Helper()

	p := New(lexer.New(input))
	p.ParseProgram()
	return p.Errors()
}

func TestLetStatements(t *testing.T) {
	tests := []struct {
		input         string
		expectedIdent string
		expectedValue string
	}{
		{"let x = 5;", "x", "5"},
		{"let y = 3.14;", "y", "3.14"},
		{"let foobar = true;", "foobar", "true"},
		{"let s = \"hi\";", "s", "\"hi\""},
		{"let n = null;", "n", "null"},
	}

	for _, tt := range tests {
		program := parse(t, tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("input %q: expected 1 statement, got %d", tt.input, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.LetStatement)
		if !ok {
			t.Fatalf("input %q: expected *ast.LetStatement, got %T", tt.input, program.Statements[0])
		}

		if stmt.Name.Value != tt.expectedIdent {
			t.Errorf("input %q: expected name %q, got %q", tt.input, tt.expectedIdent, stmt.Name.Value)
		}

		if stmt.Value.String() != tt.expectedValue {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.expectedValue, stmt.Value.String())
		}
	}
}

func TestSemicolonsAreOptional(t *testing.T) {
	program := parse(t, "let x = 5\nlet y = 6\nx + y")

	if len(program.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(program.Statements))
	}
}

func TestReturnStatements(t *testing.T) {
	program := parse(t, "return 5;\nreturn add(1, 2);\nreturn;")

	if len(program.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(program.Statements))
	}

	for i, s := range program.Statements {
		if _, ok := s.(*ast.ReturnStatement); !ok {
			t.Fatalf("statement %d: expected *ast.ReturnStatement, got %T", i, s)
		}
	}

	last := program.Statements[2].(*ast.ReturnStatement)
	if last.ReturnValue != nil {
		t.Errorf("bare `return;` should have a nil value, got %s", last.ReturnValue.String())
	}
}

func TestExpressionStatementRecordsPosition(t *testing.T) {
	program := parse(t, "let x = 1;\nx + 2;")

	es, ok := program.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected *ast.ExpressionStatement, got %T", program.Statements[1])
	}
	if es.Pos().Line != 2 {
		t.Errorf("expected expression statement on line 2, got %s", es.Pos())
	}
}

func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"-1 * 2", "((-1) * 2)"},
		{"!-a", "(!(-a))"},
		{"a + b + c", "((a + b) + c)"},
		{"a + b * c", "(a + (b * c))"},
		{"a * b + c", "((a * b) + c)"},
		{"5 % 2 * 3", "((5 % 2) * 3)"},
		{"3 + 4 * 5 == 3 * 1 + 4 * 5", "((3 + (4 * 5)) == ((3 * 1) + (4 * 5)))"},
		{"a && b || c", "((a && b) || c)"},
		{"a == b && c", "((a == b) && c)"},
		{"true == !false", "(true == (!false))"},
		{"a < b == c > d", "((a < b) == (c > d))"},
		{"1 + 2 <= 3 + 4", "((1 + 2) <= (3 + 4))"},
		{"a[0] + 1", "((a[0]) + 1)"},
		{"-a[1]", "(-(a[1]))"},
		{"add(1, 2) * 3", "(add(1, 2) * 3)"},
		{"a + b[1 + 1]", "(a + (b[(1 + 1)]))"},
	}

	for _, tt := range tests {
		program := parse(t, tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("input %q: expected 1 statement, got %d", tt.input, len(program.Statements))
		}

		got := program.Statements[0].String()
		if got != tt.expected {
			t.Errorf("input %q:\n  expected %s\n  got      %s", tt.input, tt.expected, got)
		}
	}
}

func TestIfElseExpression(t *testing.T) {
	program := parse(t, "if x > 5 { print(\"greater\"); } else { print(\"less\"); }")

	es, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected *ast.ExpressionStatement, got %T", program.Statements[0])
	}

	ie, ok := es.Expression.(*ast.IfExpression)
	if !ok {
		t.Fatalf("expected *ast.IfExpression, got %T", es.Expression)
	}

	if ie.Condition.String() != "(x > 5)" {
		t.Errorf("condition: expected (x > 5), got %s", ie.Condition)
	}
	if len(ie.Consequence.Statements) != 1 {
		t.Errorf("consequence: expected 1 statement, got %d", len(ie.Consequence.Statements))
	}
	if ie.Alternative == nil {
		t.Fatal("expected an alternative branch")
	}
	if _, ok := ie.Alternative.(*ast.BlockStatement); !ok {
		t.Errorf("alternative: expected *ast.BlockStatement, got %T", ie.Alternative)
	}
}

func TestIfWithoutElseHasNilAlternative(t *testing.T) {
	program := parse(t, "if x { 1; }")

	ie := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.IfExpression)
	if ie.Alternative != nil {
		t.Errorf("expected nil alternative, got %T", ie.Alternative)
	}
}

func TestElseIfChain(t *testing.T) {
	program := parse(t, "if a { 1; } else if b { 2; } else { 3; }")

	ie := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.IfExpression)

	nested, ok := ie.Alternative.(*ast.IfExpression)
	if !ok {
		t.Fatalf("expected else-if to nest an *ast.IfExpression, got %T", ie.Alternative)
	}
	if nested.Condition.String() != "b" {
		t.Errorf("nested condition: expected b, got %s", nested.Condition)
	}
	if _, ok := nested.Alternative.(*ast.BlockStatement); !ok {
		t.Errorf("nested alternative: expected *ast.BlockStatement, got %T", nested.Alternative)
	}
}

func TestWhileStatement(t *testing.T) {
	program := parse(t, "while i < 10 { i = i + 1; }")

	ws, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("expected *ast.WhileStatement, got %T", program.Statements[0])
	}
	if ws.Condition.String() != "(i < 10)" {
		t.Errorf("condition: expected (i < 10), got %s", ws.Condition)
	}
	if len(ws.Body.Statements) != 1 {
		t.Errorf("body: expected 1 statement, got %d", len(ws.Body.Statements))
	}
}

func TestForStatement(t *testing.T) {
	program := parse(t, "for let i = 0; i < 10; i = i + 1 { print(i); }")

	fs, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("expected *ast.ForStatement, got %T", program.Statements[0])
	}

	if _, ok := fs.Init.(*ast.LetStatement); !ok {
		t.Errorf("init: expected *ast.LetStatement, got %T", fs.Init)
	}
	if fs.Condition == nil || fs.Condition.String() != "(i < 10)" {
		t.Errorf("condition: expected (i < 10), got %v", fs.Condition)
	}
	if _, ok := fs.Post.(*ast.ExpressionStatement); !ok {
		t.Errorf("post: expected *ast.ExpressionStatement, got %T", fs.Post)
	}
	if len(fs.Body.Statements) != 1 {
		t.Errorf("body: expected 1 statement, got %d", len(fs.Body.Statements))
	}
}

func TestForStatementWithOmittedSections(t *testing.T) {
	program := parse(t, "for ; i < 3; { i = i + 1; }")

	fs, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("expected *ast.ForStatement, got %T", program.Statements[0])
	}
	if fs.Init != nil {
		t.Errorf("init: expected nil, got %T", fs.Init)
	}
	if fs.Post != nil {
		t.Errorf("post: expected nil, got %T", fs.Post)
	}
	if fs.Condition == nil {
		t.Error("condition: expected non-nil")
	}
}

func TestFunctionLiteral(t *testing.T) {
	program := parse(t, "let add = fn(a, b) { a + b; };")

	stmt := program.Statements[0].(*ast.LetStatement)
	fn, ok := stmt.Value.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("expected *ast.FunctionLiteral, got %T", stmt.Value)
	}

	if len(fn.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(fn.Parameters))
	}
	if fn.Parameters[0].Value != "a" || fn.Parameters[1].Value != "b" {
		t.Errorf("parameter names wrong: %v, %v", fn.Parameters[0].Value, fn.Parameters[1].Value)
	}
	if len(fn.Body.Statements) != 1 {
		t.Errorf("body: expected 1 statement, got %d", len(fn.Body.Statements))
	}
}

func TestFunctionDeclarationStatement(t *testing.T) {
	program := parse(t, "fn add(a, b) { return a + b; }\nadd(1, 2);")

	fs, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected *ast.FunctionStatement, got %T", program.Statements[0])
	}
	if fs.Name.Value != "add" {
		t.Errorf("expected name add, got %q", fs.Name.Value)
	}
	if len(fs.Function.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(fs.Function.Parameters))
	}
}

func TestZeroParameterFunction(t *testing.T) {
	program := parse(t, "fn pi() { return 3.14; }")

	fs := program.Statements[0].(*ast.FunctionStatement)
	if len(fs.Function.Parameters) != 0 {
		t.Errorf("expected 0 parameters, got %d", len(fs.Function.Parameters))
	}
}

func TestCallExpressions(t *testing.T) {
	program := parse(t, "add(1, 2 * 3, 4 + 5);")

	call, ok := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected *ast.CallExpression, got %T", program.Statements[0])
	}

	if call.Function.String() != "add" {
		t.Errorf("function: expected add, got %s", call.Function)
	}
	if len(call.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(call.Arguments))
	}
	if call.Arguments[1].String() != "(2 * 3)" {
		t.Errorf("argument 1: expected (2 * 3), got %s", call.Arguments[1])
	}
}

func TestNestedAndChainedCalls(t *testing.T) {
	program := parse(t, "outer(inner(1), fn(x) { x }(2));")

	call := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	if len(call.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(call.Arguments))
	}
	if _, ok := call.Arguments[0].(*ast.CallExpression); !ok {
		t.Errorf("argument 0: expected *ast.CallExpression, got %T", call.Arguments[0])
	}
	if _, ok := call.Arguments[1].(*ast.CallExpression); !ok {
		t.Errorf("argument 1: expected an immediately-invoked *ast.CallExpression, got %T", call.Arguments[1])
	}
}

func TestArrayLiteral(t *testing.T) {
	program := parse(t, "[1, 2 * 2, 3 + 3]")

	arr, ok := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("expected *ast.ArrayLiteral, got %T", program.Statements[0])
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr.Elements))
	}
	if arr.Elements[1].String() != "(2 * 2)" {
		t.Errorf("element 1: expected (2 * 2), got %s", arr.Elements[1])
	}
}

func TestEmptyArrayLiteral(t *testing.T) {
	program := parse(t, "[]")

	arr := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.ArrayLiteral)
	if len(arr.Elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(arr.Elements))
	}
}

func TestIndexExpression(t *testing.T) {
	program := parse(t, "myArray[1 + 1]")

	idx, ok := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected *ast.IndexExpression, got %T", program.Statements[0])
	}
	if idx.Left.String() != "myArray" {
		t.Errorf("left: expected myArray, got %s", idx.Left)
	}
	if idx.Index.String() != "(1 + 1)" {
		t.Errorf("index: expected (1 + 1), got %s", idx.Index)
	}
}

func TestNestedBlocksAndScoping(t *testing.T) {
	program := parse(t, "if a { if b { 1; } }")

	outer := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.IfExpression)
	inner, ok := outer.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected nested expression statement, got %T", outer.Consequence.Statements[0])
	}
	if _, ok := inner.Expression.(*ast.IfExpression); !ok {
		t.Errorf("expected nested *ast.IfExpression, got %T", inner.Expression)
	}
}

func TestBareBlockStatement(t *testing.T) {
	program := parse(t, "{ let a = 1; a + 1; }")

	block, ok := program.Statements[0].(*ast.BlockStatement)
	if !ok {
		t.Fatalf("expected *ast.BlockStatement, got %T", program.Statements[0])
	}
	if len(block.Statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(block.Statements))
	}
}

func TestCommentsAreIgnoredByParser(t *testing.T) {
	program := parse(t, "// leading\nlet x = 1; /* inline */ x + 1; // trailing")

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(program.Statements))
	}
}

func TestParserErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"let without identifier", "let = 5;"},
		{"let without assign", "let x 5;"},
		{"let without value", "let x = ;"},
		{"infix without right operand", "1 + ;"},
		{"unclosed brace", "if x { 1;"},
		{"while without body", "while x 1;"},
		{"call with missing paren", "add(1, 2;"},
		{"else without block", "if x { 1; } else 2;"},
		{"index without close bracket", "a[1;"},
		{"function declaration without body", "fn f(a)"},
		{"for without body", "for let i = 0; i < 1; i = i + 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := parseWithErrors(t, tt.input)
			if len(errs) == 0 {
				t.Fatalf("expected parser errors for input %q, got none", tt.input)
			}
		})
	}
}

// TestThatFailedStatementsDoNotEnterTheTree guards against a nil concrete
// pointer being returned as a non-nil Statement interface: such a value would
// pass a `stmt == nil` check, land in the AST, and panic when evaluated.
func TestThatFailedStatementsDoNotEnterTheTree(t *testing.T) {
	inputs := []string{
		"let = 5;", "let x 5;", "let x = ;", "1 + ;", "let", "let let",
		"let = 5;\nlet y = ;", "return return;", "{ let = 1; }", "if = 1 { }",
		"while 1", "for let = 0; ; { }", "fn f(a) 1;",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			p := New(lexer.New(input))
			program := p.ParseProgram()

			for i, stmt := range program.Statements {
				if stmt == nil {
					t.Fatalf("statement %d is nil for input %q", i, input)
				}
			}
		})
	}
}

func TestParserErrorsCarryPositions(t *testing.T) {
	errs := parseWithErrors(t, "let x = 1;\nlet y = ;")
	if len(errs) == 0 {
		t.Fatal("expected an error")
	}
	// The error should point at line 2, not line 1.
	found := false
	for _, e := range errs {
		if len(e) > 0 && e[0] == '2' {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an error prefixed with line 2, got %v", errs)
	}
}

// TestThatParserDoesNotPanicOnGarbage runs a pile of malformed inputs through
// the parser to make sure it always reports errors instead of crashing.
func TestThatParserDoesNotPanicOnGarbage(t *testing.T) {
	inputs := []string{
		"", ";", "}", "{", ")", "(", "let", "let let", "fn", "fn fn",
		"1 +", "+ 1", "if", "else", "while", "for", "return return",
		"a[", "a[]", "a[1][", "[1,", "f(", "f()", "fn(){}()", "!",
		"1 = 2", "let 1 = 2", "let x = = 2", "if x {} else {} else {}",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked on %q: %v", input, r)
				}
			}()

			p := New(lexer.New(input))
			p.ParseProgram()
		})
	}
}
