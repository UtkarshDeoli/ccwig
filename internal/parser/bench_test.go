package parser

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/lexer"
)

// benchSource exercises every precedence level and statement form, so the
// benchmark reflects the parser as a whole rather than a single rule.
const benchSource = `
let xs = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
let total = 0;

fn fib(n) {
    if n < 2 { return n; }
    return fib(n - 1) + fib(n - 2);
}

let classify = fn(n) {
    if n < 0 {
        return "negative";
    } else if n == 0 {
        return "zero";
    } else {
        return "positive";
    }
};

for let i = 0; i < len(xs); i = i + 1 {
    total = total + xs[i] * 2 - 1;
}

while total > 100 {
    total = total / 2;
    let label = "value: \t" + str(total);
    print(label, total >= 0 && !false || xs[0] <= 1);
}
`

func BenchmarkParseProgram(b *testing.B) {
	b.SetBytes(int64(len(benchSource)))

	for i := 0; i < b.N; i++ {
		p := New(lexer.New(benchSource))
		p.ParseProgram()
		if errs := p.Errors(); len(errs) != 0 {
			b.Fatalf("parser errors: %v", errs)
		}
	}
}

// BenchmarkParseExpression measures the expression grammar alone, excluding
// statement-level bookkeeping.
func BenchmarkParseExpression(b *testing.B) {
	const src = `1 + 2 * 3 - 4 / 5 % 6 < 7 == 8 && 9 || f(a, b)[c]`

	b.SetBytes(int64(len(src)))

	for i := 0; i < b.N; i++ {
		p := New(lexer.New(src))
		p.parseExpression(precLowest)
	}
}
