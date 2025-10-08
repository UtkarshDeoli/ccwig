// Package ast defines the abstract syntax tree produced by the parser.
//
// Every node carries the token that introduced it so that later phases can
// report errors with a source position. Nodes also implement String, which
// renders the tree unambiguously (with explicit parentheses) and is used by
// the REPL, by tests, and by the --ast debug flag.
package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// Node is any element of the syntax tree.
type Node interface {
	// TokenLiteral returns the literal of the token that introduced the node.
	TokenLiteral() string
	// Pos returns the source position of the token that introduced the node.
	Pos() token.Position
	// String renders the node, with explicit grouping parentheses.
	String() string
}

// Statement is a node that appears in a statement position.
type Statement interface {
	Node
	statementNode()
}

// Expression is a node that produces a value.
type Expression interface {
	Node
	expressionNode()
}

// Program is the root of every parsed file.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

// Pos returns the position of the program's first statement, or the zero
// position for an empty program.
func (p *Program) Pos() token.Position {
	if len(p.Statements) > 0 {
		return p.Statements[0].Pos()
	}
	return token.Position{}
}

// String renders every statement in the program separated by newlines.
func (p *Program) String() string {
	var out strings.Builder
	for i, s := range p.Statements {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(s.String())
	}
	return out.String()
}

// ---------------------------------------------------------------- statements

// LetStatement binds a name to a value: `let x = 5;`
type LetStatement struct {
	Token token.Token // the `let` token
	Name  *Identifier
	Value Expression
}

func (s *LetStatement) statementNode()       {}
func (s *LetStatement) TokenLiteral() string { return s.Token.Literal }
func (s *LetStatement) Pos() token.Position  { return s.Token.Pos }
func (s *LetStatement) String() string {
	return fmt.Sprintf("let %s = %s;", s.Name.Value, s.Value.String())
}

// ReturnStatement returns a value from the enclosing function: `return x;`
type ReturnStatement struct {
	Token       token.Token // the `return` token
	ReturnValue Expression  // nil for a bare `return;`
}

func (s *ReturnStatement) statementNode()       {}
func (s *ReturnStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ReturnStatement) Pos() token.Position  { return s.Token.Pos }
func (s *ReturnStatement) String() string {
	if s.ReturnValue == nil {
		return "return;"
	}
	return "return " + s.ReturnValue.String() + ";"
}

// ExpressionStatement is an expression used as a statement.
type ExpressionStatement struct {
	Token      token.Token // the first token of the expression
	Expression Expression
}

func (s *ExpressionStatement) statementNode()       {}
func (s *ExpressionStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ExpressionStatement) Pos() token.Position  { return s.Token.Pos }
func (s *ExpressionStatement) String() string {
	if s.Expression == nil {
		return ""
	}

	// The semicolon is not decoration: it is what stops this statement from
	// running into the next one when several are rendered together.
	//
	// The grammar has no newline rule, so an expression ends only where the
	// following token cannot continue it. Two adjacent expression statements
	// rendered without a separator would therefore be read back as one:
	// `(a = 1)` followed by `(b = 2)` is a call of the first, not two
	// statements.
	return s.Expression.String() + ";"
}

// BlockStatement is a brace-delimited sequence of statements and introduces a
// new scope.
type BlockStatement struct {
	Token      token.Token // the `{` token
	Statements []Statement
}

func (s *BlockStatement) statementNode()       {}
func (s *BlockStatement) TokenLiteral() string { return s.Token.Literal }
func (s *BlockStatement) Pos() token.Position  { return s.Token.Pos }
func (s *BlockStatement) String() string {
	var out strings.Builder
	out.WriteString("{ ")
	for _, stmt := range s.Statements {
		out.WriteString(stmt.String())
		out.WriteString(" ")
	}
	out.WriteString("}")
	return out.String()
}

// WhileStatement loops while Condition is truthy: `while x < 10 { ... }`
type WhileStatement struct {
	Token     token.Token // the `while` token
	Condition Expression
	Body      *BlockStatement
}

func (s *WhileStatement) statementNode()       {}
func (s *WhileStatement) TokenLiteral() string { return s.Token.Literal }
func (s *WhileStatement) Pos() token.Position  { return s.Token.Pos }
func (s *WhileStatement) String() string {
	return fmt.Sprintf("while %s %s", s.Condition.String(), s.Body.String())
}

// ForStatement is a C-style loop. Init and Post may be nil when the
// corresponding clause is omitted; Condition may be nil, meaning "always true".
type ForStatement struct {
	Token     token.Token // the `for` token
	Init      Statement
	Condition Expression
	Post      Statement
	Body      *BlockStatement
}

func (s *ForStatement) statementNode()       {}
func (s *ForStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ForStatement) Pos() token.Position  { return s.Token.Pos }

// String renders the loop in the language's own `for` syntax, so that the
// rendering can be read back by the parser.
//
// Each clause is separated by the semicolon that the syntax requires, and an
// omitted clause renders as an empty clause rather than disappearing. Both
// matter: `for ; cond; post` has to keep the separators to say that the init
// clause is absent, and a statement clause brings its own trailing semicolon
// which must not be mistaken for a clause separator.
func (s *ForStatement) String() string {
	header := fmt.Sprintf("for %s; %s; %s",
		clauseString(s.Init), exprString(s.Condition), clauseString(s.Post))

	// An omitted post clause leaves the header ending in a space, which would
	// otherwise double up against the one before the body.
	return strings.TrimRight(header, " ") + " " + s.Body.String()
}

// FunctionStatement is a named function declaration:
//
//	fn add(a, b) { return a + b; }
//
// It is sugar for `let add = fn(a, b) { ... };` and exists so that recursive
// functions can be written without forward declarations.
type FunctionStatement struct {
	Token    token.Token // the `fn` token
	Name     *Identifier
	Function *FunctionLiteral
}

func (s *FunctionStatement) statementNode()       {}
func (s *FunctionStatement) TokenLiteral() string { return s.Token.Literal }
func (s *FunctionStatement) Pos() token.Position  { return s.Token.Pos }
func (s *FunctionStatement) String() string {
	return fmt.Sprintf("fn %s%s %s", s.Name.Value, paramsString(s.Function.Parameters), s.Function.Body.String())
}

// --------------------------------------------------------------- expressions

// Identifier is a name reference.
type Identifier struct {
	Token token.Token
	Value string
}

func (e *Identifier) expressionNode()      {}
func (e *Identifier) TokenLiteral() string { return e.Token.Literal }
func (e *Identifier) Pos() token.Position  { return e.Token.Pos }
func (e *Identifier) String() string       { return e.Value }

// IntegerLiteral is a whole number such as 42.
type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (e *IntegerLiteral) expressionNode()      {}
func (e *IntegerLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *IntegerLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *IntegerLiteral) String() string       { return strconv.FormatInt(e.Value, 10) }

// FloatLiteral is a number with a fractional part such as 3.14.
type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (e *FloatLiteral) expressionNode()      {}
func (e *FloatLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *FloatLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *FloatLiteral) String() string       { return formatFloat(e.Value) }

// formatFloat renders a float as a CCWIG float literal.
//
// Go's shortest form cannot be used here: above a certain magnitude it switches
// to an exponent, and this language has none, so 1000000.0 was rendered as
// "1e+06" — which the parser read back as the integer 1 followed by "e + 6".
// The form used below never has an exponent, whatever the magnitude.
//
// A whole number is still written with a fractional part, so that a float
// always reads back as a float. Without it 1000000.0 would render as
// "1000000", an INTEGER, and re-reading a rendering would silently change the
// type of the value.
//
// The precision is the shortest that round-trips, so the fractional part is
// ".0" for a whole number and carries no trailing noise otherwise.
//
// Only finite values can reach here. A float in the tree is always a value
// written in the source, never the result of arithmetic, and a literal larger
// than a float can hold is rejected by the parser rather than read as an
// infinity.
func formatFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}

// StringLiteral is a double-quoted string.
type StringLiteral struct {
	Token token.Token
	Value string
}

func (e *StringLiteral) expressionNode()      {}
func (e *StringLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *StringLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *StringLiteral) String() string       { return quoteString(e.Value) }

// quoteString renders a string as a CCWIG string literal.
//
// Go's own quoting cannot be used here: it writes a NUL as \x00 and a bell as
// \a, and this language defines neither escape, so the rendered source was
// rejected by its own lexer. Only the escapes the language actually has may
// appear, and anything else is written through as the character itself, which
// the lexer accepts verbatim.
func quoteString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)

	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case 0:
			b.WriteString(`\0`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')

	return b.String()
}

// BooleanLiteral is `true` or `false`.
type BooleanLiteral struct {
	Token token.Token
	Value bool
}

func (e *BooleanLiteral) expressionNode()      {}
func (e *BooleanLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *BooleanLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *BooleanLiteral) String() string       { return strconv.FormatBool(e.Value) }

// NullLiteral is `null`.
type NullLiteral struct {
	Token token.Token
}

func (e *NullLiteral) expressionNode()      {}
func (e *NullLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *NullLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *NullLiteral) String() string       { return "null" }

// PrefixExpression applies a unary operator: `-x`, `!ok`.
type PrefixExpression struct {
	Token    token.Token // the prefix operator
	Operator string
	Right    Expression
}

func (e *PrefixExpression) expressionNode()      {}
func (e *PrefixExpression) TokenLiteral() string { return e.Token.Literal }
func (e *PrefixExpression) Pos() token.Position  { return e.Token.Pos }
func (e *PrefixExpression) String() string {
	return fmt.Sprintf("(%s%s)", e.Operator, e.Right.String())
}

// InfixExpression applies a binary operator: `a + b`.
type InfixExpression struct {
	Token    token.Token // the operator
	Left     Expression
	Operator string
	Right    Expression
}

func (e *InfixExpression) expressionNode()      {}
func (e *InfixExpression) TokenLiteral() string { return e.Token.Literal }
func (e *InfixExpression) Pos() token.Position  { return e.Token.Pos }
func (e *InfixExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Operator, e.Right.String())
}

// IfExpression is a conditional expression.
//
// Alternative is nil when there is no else branch, a *BlockStatement for
// `else { ... }`, or another *IfExpression for `else if ...`.
type IfExpression struct {
	Token       token.Token // the `if` token
	Condition   Expression
	Consequence *BlockStatement
	Alternative Node
}

func (e *IfExpression) expressionNode()      {}
func (e *IfExpression) TokenLiteral() string { return e.Token.Literal }
func (e *IfExpression) Pos() token.Position  { return e.Token.Pos }
func (e *IfExpression) String() string {
	out := fmt.Sprintf("if %s %s", e.Condition.String(), e.Consequence.String())
	if e.Alternative != nil {
		out += " else " + e.Alternative.String()
	}
	return out
}

// FunctionLiteral is an anonymous function: `fn(a, b) { a + b }`.
type FunctionLiteral struct {
	Token      token.Token // the `fn` token
	Parameters []*Identifier
	Body       *BlockStatement
}

func (e *FunctionLiteral) expressionNode()      {}
func (e *FunctionLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *FunctionLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *FunctionLiteral) String() string {
	return fmt.Sprintf("fn%s %s", paramsString(e.Parameters), e.Body.String())
}

// CallExpression invokes a function: `add(1, 2)`.
type CallExpression struct {
	Token     token.Token // the `(` token
	Function  Expression
	Arguments []Expression
}

func (e *CallExpression) expressionNode()      {}
func (e *CallExpression) TokenLiteral() string { return e.Token.Literal }
func (e *CallExpression) Pos() token.Position  { return e.Token.Pos }
func (e *CallExpression) String() string {
	args := make([]string, 0, len(e.Arguments))
	for _, a := range e.Arguments {
		args = append(args, a.String())
	}
	return fmt.Sprintf("%s(%s)", e.Function.String(), strings.Join(args, ", "))
}

// ArrayLiteral is a list of values: `[1, 2, 3]`.
type ArrayLiteral struct {
	Token    token.Token // the `[` token
	Elements []Expression
}

func (e *ArrayLiteral) expressionNode()      {}
func (e *ArrayLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *ArrayLiteral) Pos() token.Position  { return e.Token.Pos }
func (e *ArrayLiteral) String() string {
	elems := make([]string, 0, len(e.Elements))
	for _, el := range e.Elements {
		elems = append(elems, el.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
}

// IndexExpression reads an element from an array: `xs[0]`.
type IndexExpression struct {
	Token token.Token // the `[` token
	Left  Expression
	Index Expression
}

func (e *IndexExpression) expressionNode()      {}
func (e *IndexExpression) TokenLiteral() string { return e.Token.Literal }
func (e *IndexExpression) Pos() token.Position  { return e.Token.Pos }
func (e *IndexExpression) String() string {
	return fmt.Sprintf("(%s[%s])", e.Left.String(), e.Index.String())
}

// ------------------------------------------------------------------- helpers

func paramsString(params []*Identifier) string {
	names := make([]string, 0, len(params))
	for _, p := range params {
		names = append(names, p.Value)
	}
	return "(" + strings.Join(names, ", ") + ")"
}

// clauseString renders one clause of a `for` header, without the statement
// terminator it would normally carry.
//
// A statement renders with its own trailing semicolon, but in a loop header
// the semicolons are the clause separators, so a clause must not supply one of
// its own or the header would gain an empty clause.
func clauseString(s Statement) string {
	if s == nil {
		return ""
	}
	return strings.TrimSuffix(s.String(), ";")
}

func exprString(e Expression) string {
	if e == nil {
		return ""
	}
	return e.String()
}
