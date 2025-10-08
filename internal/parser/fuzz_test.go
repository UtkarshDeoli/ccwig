package parser

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
)

// parseSource parses src and returns the program with the parser's error list.
func parseSource(src string) (*ast.Program, []string) {
	p := New(lexer.New(src))
	program := p.ParseProgram()
	return program, p.Errors()
}

// FuzzParseProgram checks the invariants the parser must hold for any input.
//
// The most valuable of these is that String is faithful: whatever the parser
// accepts, re-parsing its rendering must produce the same tree. String backs
// the -ast flag, the REPL and the test suite, so a rendering that cannot be
// read back is a silent corruption of all three.
//
// Run with: go test ./internal/parser -fuzz FuzzParseProgram
func FuzzParseProgram(f *testing.F) {
	seeds := []string{
		"",
		"1 + 2 * 3;",
		"let x = 5;",
		"for let i = 0; i < 10; i = i + 1 { print(i); }",
		"for ; i < 10; i = i + 1 { }",
		"for let i = 0; ; i = i + 1 { }",
		"for let i = 0; i < 10; { i = i + 1; }",
		"for ;; { }",
		"fn add(a, b) { return a + b; }",
		"let f = fn(a) { a * 2; };",
		"if x { 1; } else if y { 2; } else { 3; }",
		"while x < 10 { x = x + 1; }",
		"[1, 2, 3][0];",
		"a = 1; a[0] = 2;",
		`"quoted \" and \\ and \n";`,
		"{ let a = 1; }",
		"return;",
		"fn() { };",
		"()",
		"let = 5;",
		"if",
		"}",
		"{",
		"a[",
		"f(",
		"-",
		"!",
		"1 2 3",
		"let x = ;",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		program, problems := parseSource(src)

		if program == nil {
			t.Fatal("ParseProgram returned a nil program")
		}

		// String backs every debug path, so it must survive any tree the
		// parser builds. It also walks every child, so a nil node smuggled
		// into the tree surfaces here as a panic.
		rendered := program.String()

		// Errors were reported, so the source is not expected to round-trip;
		// the parser is allowed to have dropped anything it could not read.
		if len(problems) > 0 {
			return
		}

		// The source parsed cleanly, so its rendering must be valid source
		// too, and must describe the same program.
		reparsed, reproblems := parseSource(rendered)
		if len(reproblems) != 0 {
			t.Fatalf("parser accepted %q but rejected its own rendering %q:\n  %v",
				src, rendered, reproblems)
		}
		if got := reparsed.String(); got != rendered {
			t.Fatalf("rendering is not stable:\n  src:  %q\n  once: %q\n  twice:%q",
				src, rendered, got)
		}
	})
}

// FuzzParserTerminates checks that the parser always reaches the end of its
// input. A grammar with a production that can match nothing lets the parser
// spin without consuming a token, which hangs every caller.
func FuzzParserTerminates(f *testing.F) {
	seeds := []string{
		"for ;; { }",
		"fn f() { }",
		"{{{{{{",
		"]]]]]]",
		"else else else",
		";;;;;;",
		"let let let",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		// A parse that fails to advance would loop forever rather than fail
		// an assertion, so the check is that this returns at all.
		program, _ := parseSource(src)
		_ = program.String()
	})
}
