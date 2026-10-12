package lexer

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/token"
)

func TestNextToken(t *testing.T) {
	input := `let x = 5 + 3;
let y = 3.14;
let s = "hello" + " " + "world";
let b = true && false || !null;
if x != y { x = x % 2 * 7 / 3; }
x <= 10; y >= 1; a == b;
foo_bar1(a, b);
[1, 2][0];
`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.PLUS, "+"},
		{token.INT, "3"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "y"},
		{token.ASSIGN, "="},
		{token.FLOAT, "3.14"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "s"},
		{token.ASSIGN, "="},
		{token.STRING, "hello"},
		{token.PLUS, "+"},
		{token.STRING, " "},
		{token.PLUS, "+"},
		{token.STRING, "world"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "b"},
		{token.ASSIGN, "="},
		{token.TRUE, "true"},
		{token.AND, "&&"},
		{token.FALSE, "false"},
		{token.OR, "||"},
		{token.BANG, "!"},
		{token.NULL, "null"},
		{token.SEMICOLON, ";"},

		{token.IF, "if"},
		{token.IDENT, "x"},
		{token.NE, "!="},
		{token.IDENT, "y"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.IDENT, "x"},
		{token.PERCENT, "%"},
		{token.INT, "2"},
		{token.ASTERISK, "*"},
		{token.INT, "7"},
		{token.SLASH, "/"},
		{token.INT, "3"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},

		{token.IDENT, "x"},
		{token.LE, "<="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "y"},
		{token.GE, ">="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "a"},
		{token.EQ, "=="},
		{token.IDENT, "b"},
		{token.SEMICOLON, ";"},

		{token.IDENT, "foo_bar1"},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},

		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.RBRACKET, "]"},
		{token.LBRACKET, "["},
		{token.INT, "0"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, ";"},

		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong token type. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}

	if errs := l.Errors(); len(errs) != 0 {
		t.Fatalf("lexer reported unexpected errors: %v", errs)
	}
}

func TestStringEscapes(t *testing.T) {
	input := `"a\nb\tc\"d\\e"`

	l := New(input)
	tok := l.NextToken()

	if tok.Type != token.STRING {
		t.Fatalf("expected STRING, got %q (%q)", tok.Type, tok.Literal)
	}

	const want = "a\nb\tc\"d\\e"
	if tok.Literal != want {
		t.Fatalf("escape handling wrong.\nexpected=%q\ngot=     %q", want, tok.Literal)
	}

	if errs := l.Errors(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestRemainingEscapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"carriage return", `"\r"`, "\r"},
		{"nul", `"a\0b"`, "a\x00b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != token.STRING {
				t.Fatalf("expected STRING, got %q (%q)", tok.Type, tok.Literal)
			}
			if tok.Literal != tt.want {
				t.Errorf("expected %q, got %q", tt.want, tok.Literal)
			}
			if errs := l.Errors(); len(errs) != 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestUnknownEscapeIsReported(t *testing.T) {
	l := New(`"a\qb"`)

	tok := l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected STRING, got %q (%q)", tok.Type, tok.Literal)
	}
	if len(l.Errors()) == 0 {
		t.Error("expected an error for the unknown escape \\q")
	}
}

func TestStringMayNotSpanLines(t *testing.T) {
	l := New("\"abc\ndef\"")

	if tok := l.NextToken(); tok.Type != token.ILLEGAL {
		t.Fatalf("expected ILLEGAL for a string containing a newline, got %q", tok.Type)
	}
	if len(l.Errors()) == 0 {
		t.Error("expected an error for a string spanning a line")
	}
}

func TestUnicodeIsCountedByRuneNotByte(t *testing.T) {
	l := New(`"héllo"`)

	tok := l.NextToken()
	if tok.Literal != "héllo" {
		t.Errorf("expected the literal héllo, got %q", tok.Literal)
	}

	// The closing token must be positioned by character, so a following
	// token's column is not thrown off by the multi-byte é.
	//
	// `"héllo"` is 7 runes: the two quotes and five letters. Counting bytes
	// instead would put the next token at column 9, because é is two bytes.
	next := l.NextToken()
	if next.Type != token.EOF {
		t.Fatalf("expected EOF, got %q", next.Type)
	}
	if next.Pos.Col != 8 {
		t.Errorf("expected EOF at column 8 (rune counting), got %s", next.Pos)
	}
}

func TestCommentsAreSkipped(t *testing.T) {
	input := `// a line comment
let x = 1; // trailing
/* block
   comment */
x + 2;`

	expected := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range expected {
		tok := l.NextToken()
		if tok.Type != tt.expectedType || tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - expected %q(%q), got %q(%q)",
				i, tt.expectedType, tt.expectedLiteral, tok.Type, tok.Literal)
		}
	}

	if errs := l.Errors(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestLineAndColumnTracking(t *testing.T) {
	input := "let x = 1;\nlet yy = 2;\n\n  z;"

	l := New(input)

	var toks []token.Token
	for {
		tok := l.NextToken()
		toks = append(toks, tok)
		if tok.Type == token.EOF {
			break
		}
	}

	// "yy" is on line 2. "let" starts at col 1, "yy" at col 5.
	var yy *token.Token
	var z *token.Token
	for i := range toks {
		if toks[i].Literal == "yy" {
			yy = &toks[i]
		}
		if toks[i].Literal == "z" {
			z = &toks[i]
		}
	}

	if yy == nil {
		t.Fatal("did not lex identifier yy")
	}
	if yy.Pos.Line != 2 || yy.Pos.Col != 5 {
		t.Fatalf("yy position: expected 2:5, got %s", yy.Pos)
	}

	if z == nil {
		t.Fatal("did not lex identifier z")
	}
	if z.Pos.Line != 4 || z.Pos.Col != 3 {
		t.Fatalf("z position: expected 4:3, got %s", z.Pos)
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		tok   token.Type
	}{
		{"unterminated string", `"abc`, token.ILLEGAL},
		{"malformed number", "3.", token.ILLEGAL},
		{"unterminated block comment", "/* nope", token.EOF},
		{"hex with no digits", "0x", token.ILLEGAL},
		{"binary with no digits", "0b", token.ILLEGAL},
		{"stray character", "a @ b", token.ILLEGAL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)

			sawExpected := false
			for i := 0; i < 100; i++ {
				tok := l.NextToken()
				if tok.Type == tt.tok {
					sawExpected = true
				}
				if tok.Type == token.EOF {
					break
				}
			}

			if !sawExpected {
				t.Fatalf("expected to see a %q token for input %q", tt.tok, tt.input)
			}

			if len(l.Errors()) == 0 {
				t.Fatalf("expected at least one lexer error for input %q", tt.input)
			}
		})
	}
}

func TestUnterminatedBlockCommentReportsPosition(t *testing.T) {
	l := New("let x = 1;\n/* oops")

	for {
		if l.NextToken().Type == token.EOF {
			break
		}
	}

	errs := l.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Pos.Line != 2 {
		t.Fatalf("expected error on line 2, got %s (%s)", errs[0].Pos, errs[0].Msg)
	}
}

func TestNulByteIsReportedNotSilentlyTruncated(t *testing.T) {
	l := New("let x\x00 = 1;")

	for {
		if l.NextToken().Type == token.EOF {
			break
		}
	}

	errs := l.Errors()
	if len(errs) == 0 {
		t.Fatal("expected an error for a NUL byte in the source")
	}
}

func TestBitwiseOperators(t *testing.T) {
	// `&` and `|` must not swallow the `&&` and `||` forms, and `<<` and `>>`
	// must not be read as `<` `=` or `>` `=`.
	input := `a & b | c ^ d ~e && f || g << h >> i <= j >= k < l > m`

	want := []struct {
		typ token.Type
		lit string
	}{
		{token.IDENT, "a"},
		{token.AMPERSAND, "&"},
		{token.IDENT, "b"},
		{token.PIPE, "|"},
		{token.IDENT, "c"},
		{token.CARET, "^"},
		{token.IDENT, "d"},
		{token.TILDE, "~"},
		{token.IDENT, "e"},
		{token.AND, "&&"},
		{token.IDENT, "f"},
		{token.OR, "||"},
		{token.IDENT, "g"},
		{token.SHL, "<<"},
		{token.IDENT, "h"},
		{token.SHR, ">>"},
		{token.IDENT, "i"},
		{token.LE, "<="},
		{token.IDENT, "j"},
		{token.GE, ">="},
		{token.IDENT, "k"},
		{token.LT, "<"},
		{token.IDENT, "l"},
		{token.GT, ">"},
		{token.IDENT, "m"},
	}

	l := New(input)
	for i, w := range want {
		tok := l.NextToken()
		if tok.Type != w.typ || tok.Literal != w.lit {
			t.Fatalf("token %d: want %s(%q), got %s(%q)", i, w.typ, w.lit, tok.Type, tok.Literal)
		}
	}

	if errs := l.Errors(); len(errs) != 0 {
		t.Fatalf("unexpected lexer errors: %v", errs)
	}
}

func TestRadixIntegerLiterals(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0xff", "0xff"},
		{"0XFF", "0XFF"},
		{"0b1010", "0b1010"},
		{"0o17", "0o17"},
		{"010", "010"}, // no prefix: decimal, not C's octal
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()
		if tok.Type != token.INT || tok.Literal != tt.want {
			t.Fatalf("input %q: want INT(%q), got %s(%q)", tt.input, tt.want, tok.Type, tok.Literal)
		}
	}
}

func TestRadixPrefixWithoutDigitsIsAnError(t *testing.T) {
	for _, input := range []string{"0x", "0b", "0o", "0xg"} {
		l := New(input)
		if tok := l.NextToken(); tok.Type != token.ILLEGAL {
			t.Fatalf("input %q: want ILLEGAL, got %s(%q)", input, tok.Type, tok.Literal)
		}
		if len(l.Errors()) == 0 {
			t.Fatalf("input %q: expected a lexer error", input)
		}
	}
}
