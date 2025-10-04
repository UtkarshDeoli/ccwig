// Package lexer turns CCWIG source text into a stream of tokens.
//
// The lexer works on runes rather than bytes so that source positions stay
// meaningful for non-ASCII input, and it records errors (unterminated
// strings, malformed numbers, stray characters) instead of panicking so
// that a single pass can report everything wrong with the input.
package lexer

import (
	"fmt"

	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// Error is a lexical error together with the position at which it occurred.
type Error struct {
	Msg string
	Pos token.Position
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

// Lexer scans CCWIG source text.
type Lexer struct {
	input   []rune
	pos     int  // index of ch in input
	readPos int  // index of the next rune to read
	ch      rune // current rune under examination (0 at EOF)
	line    int
	col     int
	errors  []Error
}

// New returns a Lexer positioned on the first rune of input.
func New(input string) *Lexer {
	l := &Lexer{
		input: []rune(input),
		line:  1,
		col:   0,
	}
	l.readChar()
	return l
}

// Errors returns the lexical errors accumulated so far.
func (l *Lexer) Errors() []Error {
	return l.errors
}

// NextToken returns the next token in the input, or a token of type EOF once
// the input is exhausted.
func (l *Lexer) NextToken() token.Token {
	l.skipWhitespaceAndComments()

	pos := token.Position{Line: l.line, Col: l.col}

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			return l.makeToken(token.EQ, string(ch)+string(l.ch), pos)
		}
		return l.makeToken(token.ASSIGN, "=", pos)
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			return l.makeToken(token.NE, string(ch)+string(l.ch), pos)
		}
		return l.makeToken(token.BANG, "!", pos)
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			return l.makeToken(token.LE, string(ch)+string(l.ch), pos)
		}
		return l.makeToken(token.LT, "<", pos)
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			return l.makeToken(token.GE, string(ch)+string(l.ch), pos)
		}
		return l.makeToken(token.GT, ">", pos)
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			return l.makeToken(token.AND, string(ch)+string(l.ch), pos)
		}
		l.addError(pos, "unexpected character '&' (did you mean '&&'?)")
		return l.makeToken(token.ILLEGAL, "&", pos)
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			return l.makeToken(token.OR, string(ch)+string(l.ch), pos)
		}
		l.addError(pos, "unexpected character '|' (did you mean '||'?)")
		return l.makeToken(token.ILLEGAL, "|", pos)
	case '+':
		return l.makeToken(token.PLUS, "+", pos)
	case '-':
		return l.makeToken(token.MINUS, "-", pos)
	case '*':
		return l.makeToken(token.ASTERISK, "*", pos)
	case '/':
		return l.makeToken(token.SLASH, "/", pos)
	case '%':
		return l.makeToken(token.PERCENT, "%", pos)
	case ',':
		return l.makeToken(token.COMMA, ",", pos)
	case ';':
		return l.makeToken(token.SEMICOLON, ";", pos)
	case '(':
		return l.makeToken(token.LPAREN, "(", pos)
	case ')':
		return l.makeToken(token.RPAREN, ")", pos)
	case '{':
		return l.makeToken(token.LBRACE, "{", pos)
	case '}':
		return l.makeToken(token.RBRACE, "}", pos)
	case '[':
		return l.makeToken(token.LBRACKET, "[", pos)
	case ']':
		return l.makeToken(token.RBRACKET, "]", pos)
	case '"':
		return l.readString(pos)
	case 0:
		return token.Token{Type: token.EOF, Literal: "", Pos: pos}
	}

	if isDigit(l.ch) {
		return l.readNumber(pos)
	}

	if isLetter(l.ch) {
		literal := l.readIdentifier()
		return token.Token{Type: token.LookupIdent(literal), Literal: literal, Pos: pos}
	}

	l.addError(pos, fmt.Sprintf("unexpected character %q", l.ch))
	tok := l.makeToken(token.ILLEGAL, string(l.ch), pos)
	l.readChar()
	return tok
}

func (l *Lexer) makeToken(t token.Type, literal string, pos token.Position) token.Token {
	l.readChar()
	return token.Token{Type: t, Literal: literal, Pos: pos}
}

// readChar advances to the next rune, tracking line and column. A rune value
// of 0 marks end of input, so a literal NUL byte in the source is reported as
// an error rather than being mistaken for EOF.
func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.col = 0
	}

	if l.readPos >= len(l.input) {
		l.ch = 0
		l.pos = l.readPos
		l.col++
		return
	}

	l.ch = l.input[l.readPos]
	l.pos = l.readPos
	l.readPos++
	l.col++

	if l.ch == 0 {
		l.addError(token.Position{Line: l.line, Col: l.col}, "unexpected NUL character")
	}
}

func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) readIdentifier() string {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return string(l.input[start:l.pos])
}

func (l *Lexer) readNumber(pos token.Position) token.Token {
	start := l.pos
	for isDigit(l.ch) {
		l.readChar()
	}

	if l.ch == '.' {
		l.readChar()
		if !isDigit(l.ch) {
			l.addError(pos, "malformed number: expected a digit after '.'")
			return token.Token{Type: token.ILLEGAL, Literal: "ILLEGAL", Pos: pos}
		}
		for isDigit(l.ch) {
			l.readChar()
		}
		return token.Token{Type: token.FLOAT, Literal: string(l.input[start:l.pos]), Pos: pos}
	}

	return token.Token{Type: token.INT, Literal: string(l.input[start:l.pos]), Pos: pos}
}

func (l *Lexer) readString(pos token.Position) token.Token {
	l.readChar() // consume the opening quote

	var out []rune
	for {
		if l.ch == 0 {
			l.addError(pos, "unterminated string literal")
			return token.Token{Type: token.ILLEGAL, Literal: string(out), Pos: pos}
		}
		if l.ch == '"' {
			l.readChar() // consume the closing quote
			return token.Token{Type: token.STRING, Literal: string(out), Pos: pos}
		}
		if l.ch == '\n' {
			l.addError(pos, "unterminated string literal (strings may not span lines)")
			return token.Token{Type: token.ILLEGAL, Literal: string(out), Pos: pos}
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			case '"':
				out = append(out, '"')
			case '\\':
				out = append(out, '\\')
			case '0':
				out = append(out, 0)
			case 0:
				l.addError(pos, "unterminated string literal")
				return token.Token{Type: token.ILLEGAL, Literal: string(out), Pos: pos}
			default:
				l.addError(token.Position{Line: l.line, Col: l.col}, fmt.Sprintf("unknown escape sequence \\%c", l.ch))
				out = append(out, l.ch)
			}
			l.readChar()
			continue
		}

		out = append(out, l.ch)
		l.readChar()
	}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		switch {
		case l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\n':
			l.readChar()
		case l.ch == '/' && l.peekChar() == '/':
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
		case l.ch == '/' && l.peekChar() == '*':
			start := token.Position{Line: l.line, Col: l.col}
			l.readChar() // consume '/'
			l.readChar() // consume '*'
			for {
				if l.ch == 0 {
					l.addError(start, "unterminated block comment")
					return
				}
				if l.ch == '*' && l.peekChar() == '/' {
					l.readChar() // consume '*'
					l.readChar() // consume '/'
					break
				}
				l.readChar()
			}
		default:
			return
		}
	}
}

func (l *Lexer) addError(pos token.Position, msg string) {
	l.errors = append(l.errors, Error{Msg: msg, Pos: pos})
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
