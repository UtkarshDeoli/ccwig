// Package token defines the lexical tokens of the CCWIG language and the
// source positions attached to them.
package token

import "fmt"

// Position is a 1-based location in a source file. Columns count runes,
// not bytes, so that multi-byte characters do not distort error messages.
type Position struct {
	Line int
	Col  int
}

// String renders the position as "line:col".
func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// Type identifies the kind of a token.
type Type string

// String returns a readable name for the token type. Operator and delimiter
// types store their punctuation as the type itself, so without this they
// would print as bare symbols: a token dump would show `=("=")` rather than
// `ASSIGN("=")`, and an error would read "expected ASSIGN" as "expected =".
func (t Type) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return string(t)
}

// typeNames gives readable names to the types whose own value is punctuation.
var typeNames = map[Type]string{
	ASSIGN:   "ASSIGN",
	PLUS:     "PLUS",
	MINUS:    "MINUS",
	BANG:     "BANG",
	ASTERISK: "ASTERISK",
	SLASH:    "SLASH",
	PERCENT:  "PERCENT",

	LT:  "LT",
	GT:  "GT",
	LE:  "LE",
	GE:  "GE",
	EQ:  "EQ",
	NE:  "NE",
	AND: "AND",
	OR:  "OR",

	COMMA:     "COMMA",
	SEMICOLON: "SEMICOLON",
	LPAREN:    "LPAREN",
	RPAREN:    "RPAREN",
	LBRACE:    "LBRACE",
	RBRACE:    "RBRACE",
	LBRACKET:  "LBRACKET",
	RBRACKET:  "RBRACKET",
}

// Token is a single lexical unit produced by the lexer.
type Token struct {
	Type    Type
	Literal string
	Pos     Position
}

// String renders the token for debugging and error messages.
func (t Token) String() string {
	if t.Literal == "" {
		return t.Type.String()
	}
	return fmt.Sprintf("%s(%q)", t.Type, t.Literal)
}

// The token types produced by the lexer.
const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	IDENT  = "IDENT"
	INT    = "INT"
	FLOAT  = "FLOAT"
	STRING = "STRING"

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	PERCENT  = "%"

	LT  = "<"
	GT  = ">"
	LE  = "<="
	GE  = ">="
	EQ  = "=="
	NE  = "!="
	AND = "&&"
	OR  = "||"

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACE    = "{"
	RBRACE    = "}"
	LBRACKET  = "["
	RBRACKET  = "]"

	// Keywords
	FUNCTION = "FUNCTION"
	LET      = "LET"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	NULL     = "NULL"
	IF       = "IF"
	ELSE     = "ELSE"
	RETURN   = "RETURN"
	WHILE    = "WHILE"
	FOR      = "FOR"
)

var keywords = map[string]Type{
	"fn":     FUNCTION,
	"let":    LET,
	"true":   TRUE,
	"false":  FALSE,
	"null":   NULL,
	"if":     IF,
	"else":   ELSE,
	"return": RETURN,
	"while":  WHILE,
	"for":    FOR,
}

// LookupIdent returns the keyword token type for ident, or IDENT if the
// identifier is not a reserved word.
func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
