package evaluator

import (
	"io"
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/parser"
)

// The benchmarks below measure the evaluator's hot paths. They deliberately
// re-parse nothing: the program is parsed once, outside the timed loop, so
// that the numbers reflect evaluation rather than the parser.
//
// Run them with:
//
//	go test ./internal/evaluator -bench . -benchmem

// benchProgram parses src and returns a function that evaluates it once in a
// fresh environment.
func benchProgram(b *testing.B, src string) func() {
	b.Helper()

	program := mustParse(b, src)

	return func() {
		New(io.Discard).Eval(program, object.NewEnvironment())
	}
}

func mustParse(b *testing.B, src string) *ast.Program {
	b.Helper()

	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		b.Fatalf("parser errors for %q: %v", src, errs)
	}
	return program
}

// ------------------------------------------------------------- evaluation

// BenchmarkArithmetic measures the cost of evaluating a long chain of integer
// arithmetic with no variables, so it isolates the infix path.
func BenchmarkArithmetic(b *testing.B) {
	run := benchProgram(b, "((((((((1 + 2) * 3) - 4) / 2) % 7) + 8) * 9) - 10);")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkLoopBody measures the per-iteration cost of a `for` loop, which is
// dominated by environment allocation and variable lookup.
func BenchmarkLoopBody(b *testing.B) {
	run := benchProgram(b, `
let total = 0;
for let i = 0; i < 1000; i = i + 1 {
    total = total + i;
}
total;
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkWhileLoop measures the same loop written as a `while`, which does
// not have the loop's own scope.
func BenchmarkWhileLoop(b *testing.B) {
	run := benchProgram(b, `
let i = 0;
let total = 0;
while i < 1000 {
    total = total + i;
    i = i + 1;
}
total;
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkFunctionCalls measures call overhead on a workload dominated by
// recursion, including argument binding and return unwrapping.
func BenchmarkFunctionCalls(b *testing.B) {
	run := benchProgram(b, `
fn fib(n) {
    if n < 2 { return n; }
    return fib(n - 1) + fib(n - 2);
}
fib(18);
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkArrayIndex measures indexed reads and writes in a loop, covering
// the index expression and index assignment paths.
func BenchmarkArrayIndex(b *testing.B) {
	run := benchProgram(b, `
let xs = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
let total = 0;
for let i = 0; i < 1000; i = i + 1 {
    xs[i % 10] = i;
    total = total + xs[i % 10];
}
total;
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkStringConcat measures string building, which is dominated by
// allocation in the infix path.
func BenchmarkStringConcat(b *testing.B) {
	run := benchProgram(b, `
let s = "";
for let i = 0; i < 200; i = i + 1 {
    s = s + "x";
}
len(s);
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkBuiltinCalls measures builtin dispatch, including the arity check.
func BenchmarkBuiltinCalls(b *testing.B) {
	run := benchProgram(b, `
let xs = [1, 2, 3];
let n = 0;
for let i = 0; i < 500; i = i + 1 {
    n = n + len(xs);
}
n;
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}

// BenchmarkNestedScopeLookup measures identifier resolution through several
// nested scopes, which is what a closure body pays on every variable read.
func BenchmarkNestedScopeLookup(b *testing.B) {
	run := benchProgram(b, `
fn outer() {
    let a = 1;
    return fn() {
        let b = 2;
        return fn() {
            let c = 3;
            let total = 0;
            for let i = 0; i < 200; i = i + 1 {
                total = total + a + b + c;
            }
            return total;
        };
    };
}
let f = outer()();
f();
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}
