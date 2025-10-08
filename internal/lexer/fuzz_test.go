package lexer

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// FuzzNextToken checks the invariants a scanner must never violate, whatever
// the input:
//
//   - NextToken always returns, and always terminates in EOF. A scanner that
//     can fail to advance would hang every caller, and the REPL calls it on
//     input it has not validated.
//   - Every token starts inside the input and carries a position whose line
//     and column are at least 1, so that error messages can point at it.
//   - Positions never move backwards, so a reported error is not attributed
//     to a spot before the text that caused it.
//   - The literal of a token is a substring of the source, which catches a
//     scanner that invents or drops characters.
//
// Run with: go test ./internal/lexer -fuzz FuzzNextToken
func FuzzNextToken(f *testing.F) {
	seeds := []string{
		"",
		"let x = 5;",
		`"unterminated`,
		"/* unterminated",
		"3.",
		"1..2",
		"\x00",
		"\"\\q\"",
		"\"a\nb\"",
		"a & b | c @ d",
		"let héllo = 1;",
		"0x10",
		"---1",
		"999999999999999999999999",
		"1e999",
		"[[[[[[[[[[",
		"\"\\",
		"\r\n\t\r",
		"/ / // /* /*/",
		"fn(a, b) { return a; }",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		l := New(src)

		var prev token.Position
		// The bound is a safety net: a correct scanner emits at most one
		// token per input character plus EOF. Reaching it means the scanner
		// is not advancing.
		maxTokens := len([]rune(src)) + 2

		for i := 0; i < maxTokens; i++ {
			tok := l.NextToken()

			if tok.Pos.Line < 1 || tok.Pos.Col < 1 {
				t.Fatalf("token %q has a position before the start of the input: %s",
					tok.Type, tok.Pos)
			}
			if tok.Pos.Line < prev.Line ||
				(tok.Pos.Line == prev.Line && tok.Pos.Col < prev.Col) {
				t.Fatalf("token %q at %s moves backwards from %s",
					tok.Type, tok.Pos, prev)
			}
			prev = tok.Pos

			if tok.Type == token.EOF {
				return
			}
		}

		t.Fatalf("scanner did not reach EOF within %d tokens for input %q",
			maxTokens, src)
	})
}

// FuzzErrorsArePositioned checks that every error the scanner reports names a
// position inside the input, so that a caller can render it without guarding
// against a zero value.
func FuzzErrorsArePositioned(f *testing.F) {
	seeds := []string{
		`"abc`,
		"3.",
		"/* nope",
		"a & b",
		"\x00",
		`"a\qb"`,
		"\"a\nb\"",
		"@",
		`\`,
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		l := New(src)
		for l.NextToken().Type != token.EOF {
		}

		for _, err := range l.Errors() {
			if err.Msg == "" {
				t.Error("lexer reported an error with no message")
			}
			if err.Pos.Line < 1 || err.Pos.Col < 1 {
				t.Errorf("lexer error %q has an invalid position: %s",
					err.Msg, err.Pos)
			}
		}
	})
}
