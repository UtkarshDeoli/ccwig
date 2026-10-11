package parser

import (
	"fmt"
	"strings"
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

		// Bitwise operators, placed as C places them: the shifts bind tighter
		// than the comparisons, and `&`, `^`, `|` sit looser than `==` but
		// tighter than `&&`.
		{"-a", "(-a)"},
		{"~a", "(~a)"},
		{"~a + b", "((~a) + b)"},
		{"a << b", "(a << b)"},
		{"a << b + c", "(a << (b + c))"},
		{"1 + 2 << 3", "((1 + 2) << 3)"},
		{"a << b < c", "((a << b) < c)"},
		{"a == b & c", "((a == b) & c)"},
		{"a & b == c", "(a & (b == c))"},
		{"a & b | c", "((a & b) | c)"},
		{"a | b ^ c & d", "(a | (b ^ (c & d)))"},
		{"a && b | c", "(a && (b | c))"},
		{"a | b && c", "((a | b) && c)"},
	}

	for _, tt := range tests {
		program := parse(t, tt.input)

		if len(program.Statements) != 1 {
			t.Fatalf("input %q: expected 1 statement, got %d", tt.input, len(program.Statements))
		}

		// This test is about how expressions group, so it renders the
		// expression itself. The statement around it also carries a
		// terminating semicolon, which is not what is under test here.
		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("input %q: expected an expression statement, got %T", tt.input, program.Statements[0])
		}

		got := stmt.Expression.String()
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

// ------------------------------------------------- for-loop rendering
//
// A `for` header has three clauses that are individually optional, so a
// rendering that just concatenates them cannot say which were present. The
// tests below pin down a rendering that is both readable back by this parser
// and faithful about which clauses the source actually had.

// TestForStatementRenderingKeepsItsClauses is the regression guard for a `for`
// header that rendered as `for (let i = 0; (i < 10) (i = (i + 1)))`: the
// condition and post clauses ran together with no separator, and an omitted
// clause left no trace at all.
func TestForStatementRenderingKeepsItsClauses(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"all clauses", "for let i = 0; i < 10; i = i + 1 { print(i); }"},
		{"no init", "let i = 0; for ; i < 10; i = i + 1 { print(i); }"},
		{"no condition", "for let i = 0; ; i = i + 1 { if i > 9 { return; } }"},
		{"no post", "for let i = 0; i < 10; { i = i + 1; }"},
		{"no clauses at all", "for ; ; { return; }"},
		{"assign post", "for let i = 0; i < 3; i = i + 1 { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := parse(t, tt.src)
			rendered := original.String()

			// The rendering must be source this parser accepts.
			reparsed, errs := parseSource(rendered)
			if len(errs) != 0 {
				t.Fatalf("parser rejected its own rendering of %q:\n  rendering: %s\n  errors: %v",
					tt.src, rendered, errs)
			}

			// ... and must describe the same program, so that rendering is
			// stable under repetition.
			if again := reparsed.String(); again != rendered {
				t.Errorf("rendering is not stable:\n  source: %s\n  once:   %s\n  twice:  %s",
					tt.src, rendered, again)
			}

			// The clause structure must survive: a reader of the rendering
			// has to be able to tell which clauses the original had.
			want := loopClauses(t, original)
			got := loopClauses(t, reparsed)
			if want != got {
				t.Errorf("%s: rendering changed which clauses are present\n  source:    %s\n  rendering: %s\n  want clauses %s, got %s",
					tt.name, tt.src, rendered, want, got)
			}
		})
	}
}

// loopClauses describes which clauses of the first `for` loop in a program are
// present, so that a rendering can be checked for faithfulness.
func loopClauses(t *testing.T, program *ast.Program) string {
	t.Helper()

	var loop *ast.ForStatement
	for _, stmt := range program.Statements {
		if s, ok := stmt.(*ast.ForStatement); ok {
			loop = s
			break
		}
	}
	if loop == nil {
		t.Fatalf("program has no for statement: %s", program.String())
	}

	clause := func(present bool) string {
		if present {
			return "present"
		}
		return "absent"
	}
	return fmt.Sprintf("init=%s cond=%s post=%s",
		clause(loop.Init != nil), clause(loop.Condition != nil), clause(loop.Post != nil))
}

// TestDeeplyNestedInputIsRejected guards the parser's recursion. Parsing
// `((((1))))` recurses once per level, and Go cannot recover from a stack
// exhaustion: it is a fatal error that kills the process, taking a
// long-running shell or editor session with it. The parser must therefore
// refuse to descend rather than discover the limit by dying.
func TestDeeplyNestedInputIsRejected(t *testing.T) {
	const depth = 5000

	src := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)

	p := New(lexer.New(src))
	program := p.ParseProgram()

	if errs := p.Errors(); len(errs) == 0 {
		t.Errorf("expected nesting %d deep to be rejected, but it parsed cleanly", depth)
	}
	if program == nil {
		t.Fatal("ParseProgram returned nil")
	}
}

// TestExtremeNestingDoesNotCrashTheProcess exercises nesting far past any
// plausible limit. Before the parser bounded its recursion this input
// exhausted the goroutine stack and aborted the process with a runtime dump.
func TestExtremeNestingDoesNotCrashTheProcess(t *testing.T) {
	const depth = 2_000_000

	src := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)

	p := New(lexer.New(src))
	p.ParseProgram()

	if errs := p.Errors(); len(errs) == 0 {
		t.Errorf("expected nesting %d deep to be rejected", depth)
	}
}

// TestNestingDepthLimitReportsOneError keeps the depth error readable. Every
// level of an over-deep expression would otherwise record its own copy, so a
// reader would be shown thousands of identical lines instead of one.
func TestNestingDepthLimitReportsOneError(t *testing.T) {
	const depth = 5000

	src := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)

	p := New(lexer.New(src))
	p.ParseProgram()

	errs := p.Errors()
	if len(errs) == 0 {
		t.Fatal("expected an error for excessively deep nesting")
	}
	if len(errs) > 3 {
		t.Errorf("expected at most a few errors for deep nesting, got %d: %v", len(errs), errs)
	}
}

// TestNestingWithinTheLimitIsAccepted makes sure the depth limit does not
// reject expressions that are deeply nested but perfectly reasonable, and
// that a long left-associative chain is not mistaken for deep nesting.
func TestNestingWithinTheLimitIsAccepted(t *testing.T) {
	t.Run("nested parentheses", func(t *testing.T) {
		const depth = 200
		src := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)

		p := New(lexer.New(src))
		p.ParseProgram()

		if errs := p.Errors(); len(errs) != 0 {
			t.Errorf("nesting %d deep should be accepted, got %v", depth, errs)
		}
	})

	t.Run("long left-associative chain", func(t *testing.T) {
		// `1 + 1 + 1 + ...` groups to the left, so it must not be mistaken
		// for deep nesting however long it gets.
		src := strings.TrimSuffix(strings.Repeat("1 + ", 20000), " + ") + ";"

		p := New(lexer.New(src))
		p.ParseProgram()

		if errs := p.Errors(); len(errs) != 0 {
			t.Errorf("a 20000 term sum should be accepted, got %v", errs)
		}
	})
}

// TestProgramRenderingKeepsStatementsApart is the regression guard for a
// rendering that silently merged two statements into one.
//
// The grammar has no newline rule: an expression ends only where the next
// token cannot continue it. So `a = 1; a[0] = 2;` rendered as `(a = 1)` on one
// line and `((a[0]) = 2)` on the next was read back as a single call of the
// first by the second.
func TestProgramRenderingKeepsStatementsApart(t *testing.T) {
	// want is the number of top-level statements in src, spelled out rather
	// than counted from the text: a `;` appears inside loop headers and block
	// bodies too, so splitting the source on semicolons gets it wrong.
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"assignments", "a = 1; a[0] = 2;", 2},
		{"two calls", "f(); g();", 2},
		{"call then group", "f(); (1 + 2);", 2},
		{"group then call", "(1 + 2); f();", 2},
		{"index then call", "a[0]; f();", 2},
		{"call then index", "f(); a[0];", 2},
		{"literal then call", "1; f();", 2},
		{"bare identifiers", "a; b;", 2},
		{
			"statements of every kind",
			"let x = 1; x = 2; f(); { x = 3; } while x < 5 { x = x + 1; } for ;; { } fn g() { } g();",
			8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := parse(t, tt.src)
			if got := len(original.Statements); got != tt.want {
				t.Fatalf("test is wrong: %q parsed to %d statements, expected %d",
					tt.src, got, tt.want)
			}

			rendered := original.String()

			reparsed, errs := parseSource(rendered)
			if len(errs) != 0 {
				t.Fatalf("parser rejected its own rendering of %q:\n  rendering: %s\n  errors: %v",
					tt.src, rendered, errs)
			}

			if got := len(reparsed.Statements); got != tt.want {
				t.Errorf("rendering split or merged statements:\n  source:    %s\n  rendering: %s\n  want %d statements, re-read %d",
					tt.src, rendered, tt.want, got)
			}
			if again := reparsed.String(); again != rendered {
				t.Errorf("rendering is not stable:\n  source: %s\n  once:   %s\n  twice:  %s",
					tt.src, rendered, again)
			}
		})
	}
}

// TestStringRenderingUsesTheLanguagesOwnEscapes is the regression guard for a
// string literal rendered with Go's quoting rules instead of CCWIG's.
//
// A string holding a NUL was rendered as "\x00", which this lexer rejects with
// "unknown escape sequence \x": the parser accepted the source, then could not
// read back its own rendering of it.
func TestStringRenderingUsesTheLanguagesOwnEscapes(t *testing.T) {
	tests := []struct {
		name string
		// src is the literal as written in CCWIG source.
		src string
	}{
		{"nul escape", `"a\0b"`},
		{"nul at the start", `"\0abc"`},
		{"nul at the end", `"abc\0"`},
		{"only a nul", `"\0"`},
		{"backslash", `"a\\b"`},
		{"quote", `"a\"b"`},
		{"newline", `"a\nb"`},
		{"tab", `"a\tb"`},
		{"carriage return", `"a\rb"`},
		{"every escape together", `"\n\t\r\"\\\0"`},
		{"raw control character", "\"a\x07b\""},
		{"raw delete character", "\"a\x7fb\""},
		{"non-ascii text", `"héllo ☃"`},
		{"looks like an escape", `"\n not a newline"`},
		{"empty", `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := parse(t, tt.src)
			rendered := original.String()

			// The rendering must be source this parser accepts.
			reparsed, errs := parseSource(rendered)
			if len(errs) != 0 {
				t.Fatalf("parser rejected its own rendering of %q:\n  rendering: %q\n  errors: %v",
					tt.src, rendered, errs)
			}

			// ... and must describe the same text.
			want := literalValue(t, original)
			got := literalValue(t, reparsed)
			if got != want {
				t.Errorf("rendering changed the string:\n  source:    %q\n  rendering: %q\n  want value %q, got %q",
					tt.src, rendered, want, got)
			}

			if again := reparsed.String(); again != rendered {
				t.Errorf("rendering is not stable:\n  source: %q\n  once:   %q\n  twice:  %q",
					tt.src, rendered, again)
			}
		})
	}
}

// TestFloatRenderingStaysReadable is the regression guard for a float literal
// rendered in a notation the language cannot read back.
//
// Rendering used Go's shortest form, which switches to an exponent once the
// magnitude is large enough: 1000000.0 came out as "1e+06". This lexer has no
// exponent syntax, so the parser could not read its own rendering — "1e+06"
// re-parsed as the integer 1 followed by "e + 6", an entirely different
// program. The same formatter wrote a whole-valued float as "1000000", which
// reads back as an INTEGER, quietly changing the value's type.
func TestFloatRenderingStaysReadable(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"the magnitude that first exposed it", "1000000.0;"},
		{"a million written out", "1000000.5;"},
		{"ordinary fraction", "3.5;"},
		{"less than one", "0.25;"},
		{"zero", "0.0;"},
		{"negative zero", "-0.0;"},
		{"negative", "-2.5;"},
		{"negative million", "-1000000.0;"},
		{"large enough to reach for an exponent", "12345678901234567890.0;"},
		{"small enough to reach for an exponent", "0.0000000001;"},
		{"smallest denormal", "0.000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005;"},
		{"trailing fraction", "1.0000001;"},
		{"many digits", "3.141592653589793;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := parse(t, tt.src)
			want := floatValue(t, soleExpression(t, original))
			rendered := original.String()

			reparsed, errs := parseSource(rendered)
			if len(errs) != 0 {
				t.Fatalf("parser rejected its own rendering of %q:\n  rendering: %s\n  errors: %v",
					tt.src, rendered, errs)
			}

			// The rendering has to be a float literal again, not just any node:
			// a form that reads back as an integer has changed the program.
			got := floatValue(t, soleExpression(t, reparsed))
			if got != want {
				t.Errorf("rendering changed the value:\n  source:    %s\n  rendering: %s\n  want %v, got %v",
					tt.src, rendered, want, got)
			}

			if again := reparsed.String(); again != rendered {
				t.Errorf("rendering is not stable:\n  source: %s\n  once:   %s\n  twice:  %s",
					tt.src, rendered, again)
			}
		})
	}
}

// TestExtremeFloatLiteralsRoundTrip pins the two facts that make rendering a
// float in plain decimal notation safe at any magnitude.
//
// A literal too large for a float is a parse error rather than an infinity, so
// the tree only ever holds finite values and the renderer never has to write
// "+Inf". And a literal at the very top of the range still round-trips: it is
// written out as 309 plain digits, with no exponent for the parser to choke on.
func TestExtremeFloatLiteralsRoundTrip(t *testing.T) {
	t.Run("a literal beyond the range of a float is rejected", func(t *testing.T) {
		src := strings.Repeat("9", 400) + ".0;"

		_, errs := parseSource(src)
		if len(errs) == 0 {
			t.Fatal("a float literal too large to represent was accepted")
		}
		if !strings.Contains(errs[0], "as a number") {
			t.Errorf("unexpected error for an out-of-range literal: %s", errs[0])
		}
	})

	t.Run("a literal at the top of the range round-trips", func(t *testing.T) {
		// 1 followed by 308 zeros: finite, and the largest power of ten a float
		// can hold exactly enough of.
		src := "1" + strings.Repeat("0", 308) + ".0;"

		original := parse(t, src)
		rendered := original.String()

		if strings.ContainsAny(rendered, "eE") {
			t.Fatalf("a large float was rendered in a notation the lexer cannot read: %s", rendered)
		}

		reparsed, errs := parseSource(rendered)
		if len(errs) != 0 {
			t.Fatalf("parser rejected its own rendering:\n  rendering: %s\n  errors: %v", rendered, errs)
		}

		if got, want := floatValue(t, soleExpression(t, reparsed)), floatValue(t, soleExpression(t, original)); got != want {
			t.Errorf("rendering changed the value: want %v, got %v", want, got)
		}
	})
}

// soleExpression returns the expression of a program's only statement.
func soleExpression(t *testing.T, program *ast.Program) ast.Expression {
	t.Helper()

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected an expression statement, got %T", program.Statements[0])
	}
	return stmt.Expression
}

// floatValue returns the value of a float literal, unwrapping a unary minus.
//
// It insists on finding a *ast.FloatLiteral: a rendering that reads back as an
// integer is a failure even when the numeric value is unchanged, because the
// program it describes is no longer the same one.
func floatValue(t *testing.T, e ast.Expression) float64 {
	t.Helper()

	negated := false
	if p, ok := e.(*ast.PrefixExpression); ok {
		if p.Operator != "-" {
			t.Fatalf("expected a float literal or its negation, got %q applied to %T", p.Operator, p.Right)
		}
		negated = true
		e = p.Right
	}

	lit, ok := e.(*ast.FloatLiteral)
	if !ok {
		t.Fatalf("expected a float literal, got %T: the rendering is no longer a float", e)
	}
	if negated {
		return -lit.Value
	}
	return lit.Value
}

// literalValue returns the text of the single string literal in a program.
func literalValue(t *testing.T, program *ast.Program) string {
	t.Helper()

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected an expression statement, got %T", program.Statements[0])
	}
	lit, ok := stmt.Expression.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("expected a string literal, got %T", stmt.Expression)
	}
	return lit.Value
}
