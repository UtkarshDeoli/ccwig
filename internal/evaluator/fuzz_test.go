package evaluator

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/parser"
)

// evalForFuzz evaluates program, turning the one panic the evaluator raises on
// purpose into an ordinary error value.
//
// `exit` stops a program by panicking out of the evaluator, which is right
// when a program is running and wrong when a fuzzer is: a generated `exit()`
// would otherwise take the test binary down. Fuzz targets evaluate through
// here so that the panic is caught at the boundary the interpreter would have
// caught it at.
func evalForFuzz(ev *Evaluator, program *ast.Program) (result object.Object) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		exit, ok := r.(ExitError)
		if !ok {
			panic(r)
		}
		result = &object.Error{Message: fmt.Sprintf("exit with status %d", exit.Code)}
	}()

	return ev.Eval(program, object.NewEnvironment())
}

// fuzzMaxSteps keeps fuzzed programs cheap. A generated `while true {}` would
// otherwise burn the full 50 million step budget on every input.
const fuzzMaxSteps = 200_000

// FuzzEval checks the invariants the evaluator must hold for any input,
// whether or not it parses.
//
//   - Eval always returns a value. Returning nil from the top level would
//     dereference a nil interface in the CLI's result reporting.
//   - It never panics out of the program boundary. The evaluated node types are
//     the parser's output, but an unhandled combination must surface as an
//     error value rather than taking the process down. The one deliberate
//     panic, `exit`, is caught by evalForFuzz, which stands in for the
//     interpreter that would catch it in a real run.
//   - Every error names a position, so the CLI can render file:line:col.
//   - Inspect never panics on a returned value, since the REPL prints it.
//   - The step limit cannot be exceeded, which is what stops a generated
//     infinite loop from hanging the fuzzer.
//
// Run with: go test ./internal/evaluator -fuzz FuzzEval
func FuzzEval(f *testing.F) {
	seeds := []string{
		"",
		"1 + 2;",
		"let x = 5; x;",
		`"a" + "b";`,
		"1 + \"a\";",
		"undefined_name;",
		"[1, 2, 3][1];",
		"[1, 2, 3][99];",
		"[1, 2, 3][-1];",
		"[1, 2, 3][\"x\"];",
		"let f = fn(a) { a; }; f(1, 2);",
		"let f = fn(a) { a; }; f();",
		"1();",
		"null();",
		"while true { }",
		"for ;; { }",
		"fn f(n) { return n < 1; } f(0);",
		"-true;",
		"!1;",
		"5 % 0;",
		"5 / 0;",
		"5.0 % 2;",
		"1 < \"a\";",
		"len(1);",
		"push(1, 2);",
		"first([]);",
		"last([]);",
		"rest([]);",
		"1 = 2;",
		"f = 1;",
		"{}",
		"let a = 1; { let a = 2; } a;",
		"true && 1;",
		"null || null;",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		p := parser.New(lexer.New(src))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			return
		}

		var buf bytes.Buffer
		ev := New(&buf)
		ev.MaxSteps = fuzzMaxSteps

		result := evalForFuzz(ev, program)

		if result == nil {
			t.Fatalf("Eval returned nil for %q", src)
		}

		// A returned value is printed by the REPL and by `print`, so
		// rendering it must not fail.
		_ = result.Inspect()

		if errObj, ok := result.(*object.Error); ok {
			if errObj.Message == "" {
				t.Errorf("error value with an empty message for %q", src)
			}
			if errObj.Pos.Line < 1 || errObj.Pos.Col < 1 {
				t.Errorf("error %q has no usable position (%s) for %q",
					errObj.Message, errObj.Pos, src)
			}
		}

		if ev.steps > fuzzMaxSteps+1 {
			t.Errorf("step limit not enforced: %d steps for %q", ev.steps, src)
		}
	})
}

// FuzzEvalIsDeterministic checks that evaluating the same program twice gives
// the same result. A difference means the evaluator is carrying state across
// runs, which would make the REPL's shared environment unpredictable.
func FuzzEvalIsDeterministic(f *testing.F) {
	seeds := []string{
		"let x = 1; x;",
		"[1, 2][0];",
		"let xs = [1]; push(xs, 2); len(xs);",
		"1 + 2;",
		"undefined_name;",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		p := parser.New(lexer.New(src))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			return
		}

		run := func() string {
			ev := New(&bytes.Buffer{})
			ev.MaxSteps = fuzzMaxSteps
			return evalForFuzz(ev, program).Inspect()
		}

		first, second := run(), run()
		if first != second {
			t.Fatalf("evaluating %q twice gave %q then %q", src, first, second)
		}
	})
}

// FuzzRenderingPreservesMeaning checks that a program and the rendering of its
// syntax tree do the same thing.
//
// The parser's round-trip test checks that a rendering re-parses to the same
// rendering, which is what the REPL and `-ast` need to stay readable. This
// checks the stronger property that the user actually depends on: that the
// re-parsed program produces the same output. It is what would catch a
// rendering that is self-consistent but wrong — one that quietly merges two
// statements into a single call re-renders to itself quite happily, and only
// running the two programs side by side shows the difference.
//
// Sources that do not parse are skipped, as are programs that differ only in
// having run out of steps at different points, which is a real difference in
// behaviour and so is reported rather than skipped.
func FuzzRenderingPreservesMeaning(f *testing.F) {
	seeds := []string{
		"1 + 2;",
		"let x = 5; print(x);",
		"a = 1; a[0] = 2;",
		"print(1000000.0);",
		`print("a\0b");`,
		"for let i = 0; i < 3; i = i + 1 { print(i); }",
		"for ;; { }",
		"while false { }",
		"fn f(n) { if n < 1 { 0; } else { f(n - 1); } } print(f(5));",
		"let xs = [1, 2, 3]; print(xs[1]); xs[1] = 9; print(xs);",
		"print(len(\"héllo\"));",
		"if true { print(1); } else { print(2); }",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 4000 {
			t.Skip()
		}

		p := parser.New(lexer.New(src))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			return
		}

		rendered := program.String()
		if len(rendered) > 4*len(src)+1000 {
			t.Fatalf("rendering %q grew out of proportion: %q", src, rendered)
		}

		// The rendering has to be source the parser accepts at all.
		rp := parser.New(lexer.New(rendered))
		reread := rp.ParseProgram()
		if errs := rp.Errors(); len(errs) != 0 {
			t.Fatalf("parser rejected its own rendering of %q:\n  rendering: %s\n  errors: %v",
				src, rendered, errs)
		}

		// ... and running it has to do the same thing.
		want := runCapturing(program)
		got := runCapturing(reread)
		if want != got {
			t.Fatalf("the rendering of %q behaves differently:\n  source:    %s\n  rendering: %s\n  want: %s\n  got:  %s",
				src, src, rendered, want, got)
		}
	})
}

// runCapturing evaluates a program and returns everything it printed followed
// by the value it produced. The step budget is small so that a generated
// infinite loop costs little.
func runCapturing(program *ast.Program) string {
	var buf bytes.Buffer

	ev := New(&buf)
	ev.MaxSteps = fuzzMaxSteps

	result := evalForFuzz(ev, program)

	return buf.String() + "=> " + result.Inspect()
}
