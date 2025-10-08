package lexer

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// benchSource is a representative slice of the language: declarations, loops,
// functions, strings with escapes, arrays and comments.
const benchSource = `
// A small program used to measure scanning throughput.
let xs = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
let total = 0;

fn fib(n) {
    if n < 2 { return n; }
    return fib(n - 1) + fib(n - 2);
}

for let i = 0; i < len(xs); i = i + 1 {
    total = total + xs[i] * 2 - 1;
}

/* a block comment
   spanning lines */
while total > 100 {
    total = total / 2;
    let label = "value: \t" + str(total);
    print(label, total >= 0 && !false);
}
`

func BenchmarkNextToken(b *testing.B) {
	b.SetBytes(int64(len(benchSource)))

	for i := 0; i < b.N; i++ {
		l := New(benchSource)
		for l.NextToken().Type != token.EOF {
		}
	}
}
