// Package parser builds an AST from a stream of tokens using recursive
// descent, with a Pratt (precedence-climbing) parser for expressions.
//
// The parser never panics on malformed input: it records the problem, tries
// to resynchronise, and keeps going, so a single run can report every syntax
// error in a file.
package parser

import (
	"fmt"
	"strconv"

	"github.com/UtkarshDeoli/ccwig/internal/ast"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// Operator precedence levels, lowest binding first.
//
// The order matches C, which is the only defensible choice for a language with
// a C accent: a reader who knows where `&` sits relative to `==` should not
// have to learn this language's opinion instead. The bitwise operators sit
// between `&&` and `==`, and the shifts between the comparisons and `+`.
const (
	_ int = iota
	precLowest
	precAssign     // =
	precOr         // ||
	precAnd        // &&
	precBitOr      // |
	precBitXor     // ^
	precBitAnd     // &
	precEquals     // == !=
	precComparison // < > <= >=
	precShift      // << >>
	precSum        // + -
	precProduct    // * / %
	precPrefix     // -x !x ~x
	precCall       // f(x)
	precIndex      // a[i]
)

var precedences = map[token.Type]int{
	token.ASSIGN:    precAssign,
	token.OR:        precOr,
	token.AND:       precAnd,
	token.PIPE:      precBitOr,
	token.CARET:     precBitXor,
	token.AMPERSAND: precBitAnd,
	token.EQ:        precEquals,
	token.NE:        precEquals,
	token.LT:        precComparison,
	token.GT:        precComparison,
	token.LE:        precComparison,
	token.GE:        precComparison,
	token.SHL:       precShift,
	token.SHR:       precShift,
	token.PLUS:      precSum,
	token.MINUS:     precSum,
	token.ASTERISK:  precProduct,
	token.SLASH:     precProduct,
	token.PERCENT:   precProduct,
	token.LPAREN:    precCall,
	token.LBRACKET:  precIndex,
}

// MaxNestingDepth bounds how deeply expressions may nest.
//
// Parsing an expression recurses once per nesting level, and Go cannot recover
// from exhausting the goroutine stack: it is a fatal error that aborts the
// process rather than a panic that recover could catch. Without a limit,
// input such as two million open parentheses takes the whole process down, so
// the parser stops descending and reports the problem while it still can.
//
// The limit is far beyond anything a person writes. A left-associative chain
// such as `1 + 1 + 1 + ...` is not affected however long it is, because it
// iterates rather than recurses.
const MaxNestingDepth = 1000

// Parser turns tokens into an AST.
type Parser struct {
	l *lexer.Lexer

	curToken  token.Token
	peekToken token.Token

	errors []string

	// depth is the current expression nesting level and maxDepth the level at
	// which parsing gives up.
	depth    int
	maxDepth int
	// reportedDepth keeps an over-deep expression from recording one error per
	// level of nesting, which would bury the message under thousands of copies.
	reportedDepth bool
}

// New returns a Parser reading from l, positioned on the first token.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, maxDepth: MaxNestingDepth}

	// Read two tokens so that curToken and peekToken are both populated.
	p.nextToken()
	p.nextToken()

	// Lexical errors are syntax errors as far as the user is concerned.
	for _, err := range l.Errors() {
		p.errors = append(p.errors, err.Error())
	}

	return p
}

// Errors returns the syntax errors found while parsing.
func (p *Parser) Errors() []string {
	// Pick up any lexical errors emitted after construction (the lexer is
	// pulled lazily as parsing proceeds).
	for _, err := range p.l.Errors() {
		msg := err.Error()
		if !contains(p.errors, msg) {
			p.errors = append(p.errors, msg)
		}
	}
	return p.errors
}

// ParseProgram parses the whole input and returns the root node. Errors are
// reported through Errors rather than being returned here.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}

	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt == nil {
			// The statement failed. Skip to the end of it so that the
			// recovery does not produce a cascade of follow-on errors, each
			// one further from the real mistake.
			p.synchronize()
		} else {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

// synchronize discards tokens until it reaches the token that ends the
// current statement, or the end of input. It leaves curToken on the
// semicolon so that the caller's nextToken moves to the following statement.
func (p *Parser) synchronize() {
	for !p.curTokenIs(token.EOF) && !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

// synchronizeBlock is synchronize for statements inside a block, where a
// statement may end with a semicolon or with the closing brace itself.
func (p *Parser) synchronizeBlock() {
	for !p.curTokenIs(token.EOF) && !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.RBRACE) {
		p.nextToken()
	}
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.Type) bool  { return p.curToken.Type == t }
func (p *Parser) peekTokenIs(t token.Type) bool { return p.peekToken.Type == t }

// expectPeek advances only if the next token is of type t, otherwise it
// records an error at the current position.
func (p *Parser) expectPeek(t token.Type) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.Type) {
	p.addErrorf(p.peekToken.Pos, fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type))
}

func (p *Parser) addError(pos token.Position, format string, args ...any) {
	p.errors = append(p.errors, fmt.Sprintf("%s: %s", pos, fmt.Sprintf(format, args...)))
}

// addErrorf is addError for a pre-formatted message.
func (p *Parser) addErrorf(pos token.Position, msg string) {
	p.addError(pos, "%s", msg)
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curToken.Type]; ok {
		return prec
	}
	return precLowest
}

func (p *Parser) peekPrecedence() int {
	if prec, ok := precedences[p.peekToken.Type]; ok {
		return prec
	}
	return precLowest
}

// ---------------------------------------------------------------- statements

// parseStatement parses one statement, or returns nil if the input at the
// current token cannot start a statement.
//
// Each branch checks the concrete result for nil before returning it. The
// parse functions return concrete pointer types, and returning a nil pointer
// from a function whose result is assigned to the Statement interface would
// produce a non-nil interface holding a nil pointer — so `stmt == nil` would
// be false and the nil would travel into the AST.
func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		if stmt := p.parseLetStatement(); stmt != nil {
			return stmt
		}
	case token.RETURN:
		if stmt := p.parseReturnStatement(); stmt != nil {
			return stmt
		}
	case token.WHILE:
		if stmt := p.parseWhileStatement(); stmt != nil {
			return stmt
		}
	case token.FOR:
		if stmt := p.parseForStatement(); stmt != nil {
			return stmt
		}
	case token.LBRACE:
		// A bare block is a statement; it introduces a scope of its own.
		if stmt := p.parseBlockStatement(); stmt != nil {
			return stmt
		}
	case token.FUNCTION:
		// `fn name(...)` is a declaration; anything else starting with `fn`
		// is a function literal used as an expression.
		if p.peekTokenIs(token.IDENT) {
			if stmt := p.parseFunctionStatement(); stmt != nil {
				return stmt
			}
		} else if stmt := p.parseExpressionStatement(); stmt != nil {
			return stmt
		}
	default:
		if stmt := p.parseExpressionStatement(); stmt != nil {
			return stmt
		}
	}

	return nil
}

func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(precLowest)
	if stmt.Value == nil {
		return nil
	}

	p.consumeOptionalSemicolon()
	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	// A bare `return;` is allowed: it returns null.
	if p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.RBRACE) || p.peekTokenIs(token.EOF) {
		p.nextToken()
		p.consumeOptionalSemicolon()
		return stmt
	}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(precLowest)
	if stmt.ReturnValue == nil {
		return nil
	}

	p.consumeOptionalSemicolon()
	return stmt
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	p.nextToken()
	stmt.Condition = p.parseExpression(precLowest)
	if stmt.Condition == nil {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseForStatement parses `for [init]; [cond]; [post] { body }`. Every clause
// is optional; an omitted condition means "loop forever".
func (p *Parser) parseForStatement() *ast.ForStatement {
	stmt := &ast.ForStatement{Token: p.curToken}

	// --- init clause -------------------------------------------------------
	if !p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		init := p.parseStatement()
		if init == nil {
			return nil
		}
		stmt.Init = init

		if !p.curTokenIs(token.SEMICOLON) && !p.expectPeek(token.SEMICOLON) {
			return nil
		}
	} else {
		p.nextToken() // consume the `;` that closes the empty init clause
	}

	// --- condition clause --------------------------------------------------
	if !p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		stmt.Condition = p.parseExpression(precLowest)
		if stmt.Condition == nil {
			return nil
		}

		if !p.expectPeek(token.SEMICOLON) {
			return nil
		}
	} else {
		p.nextToken() // consume the `;` that closes the empty condition clause
	}

	// --- post clause -------------------------------------------------------
	if !p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		post := p.parseStatement()
		if post == nil {
			return nil
		}
		stmt.Post = post
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseFunctionStatement() *ast.FunctionStatement {
	stmt := &ast.FunctionStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	params, ok := p.parseFunctionParameters()
	if !ok {
		return nil
	}

	stmt.Function = &ast.FunctionLiteral{Token: stmt.Token, Parameters: params}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Function.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt == nil {
			p.synchronizeBlock()
		} else {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	if p.curTokenIs(token.EOF) {
		p.addError(block.Token.Pos, "unexpected end of input: this '{' is never closed")
	}

	return block
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(precLowest)
	if stmt.Expression == nil {
		return nil
	}

	p.consumeOptionalSemicolon()
	return stmt
}

// consumeOptionalSemicolon advances past a trailing `;` if one is present.
// Semicolons are optional in CCWIG; they are accepted so that C-style source
// looks natural.
func (p *Parser) consumeOptionalSemicolon() {
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

// --------------------------------------------------------------- expressions

// parseExpression parses an expression whose operators bind more tightly than
// precedence.
//
// It is the recursion point of the expression grammar, so it is where nesting
// is metered: a level that would take the parser past maxDepth is refused
// rather than attempted, and the refusal is reported once.
func (p *Parser) parseExpression(precedence int) ast.Expression {
	if p.depth >= p.maxDepth {
		p.reportTooDeep()
		return nil
	}

	p.depth++
	expr := p.parseExpressionLevel(precedence)
	p.depth--

	return expr
}

// reportTooDeep records that the expression nests too deeply, at most once per
// parse.
func (p *Parser) reportTooDeep() {
	if p.reportedDepth {
		return
	}
	p.reportedDepth = true
	p.addError(p.curToken.Pos, "expression nests too deeply (limit %d)", p.maxDepth)
}

func (p *Parser) parseExpressionLevel(precedence int) ast.Expression {
	prefix := p.prefixParseFn(p.curToken.Type)
	if prefix == nil {
		p.addError(p.curToken.Pos, "unexpected %s, expected an expression", describe(p.curToken))
		return nil
	}

	left := prefix()
	if left == nil {
		// The prefix parse failed, so there is no left operand. Continuing
		// into the infix loop would build a node with a nil operand, which
		// the tree would then carry until something tried to render it.
		return nil
	}

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFn(p.peekToken.Type)
		if infix == nil {
			return left
		}
		p.nextToken()
		left = infix(left)
		if left == nil {
			return nil
		}
	}

	return left
}

func (p *Parser) prefixParseFn(t token.Type) func() ast.Expression {
	switch t {
	case token.IDENT:
		return p.parseIdentifier
	case token.INT:
		return p.parseIntegerLiteral
	case token.FLOAT:
		return p.parseFloatLiteral
	case token.STRING:
		return p.parseStringLiteral
	case token.TRUE, token.FALSE:
		return p.parseBooleanLiteral
	case token.NULL:
		return p.parseNullLiteral
	case token.BANG, token.MINUS, token.TILDE:
		return p.parsePrefixExpression
	case token.LPAREN:
		return p.parseGroupedExpression
	case token.IF:
		return p.parseIfExpression
	case token.FUNCTION:
		return p.parseFunctionLiteral
	case token.LBRACKET:
		return p.parseArrayLiteral
	default:
		return nil
	}
}

func (p *Parser) infixParseFn(t token.Type) func(ast.Expression) ast.Expression {
	switch t {
	case token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.PERCENT,
		token.EQ, token.NE, token.LT, token.GT, token.LE, token.GE,
		token.AND, token.OR,
		token.AMPERSAND, token.PIPE, token.CARET, token.SHL, token.SHR:
		return p.parseInfixExpression
	case token.LPAREN:
		return p.parseCallExpression
	case token.LBRACKET:
		return p.parseIndexExpression
	case token.ASSIGN:
		return p.parseAssignExpression
	default:
		return nil
	}
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := parseInteger(p.curToken.Literal)
	if err != nil {
		p.addError(p.curToken.Pos, "could not parse %q as an integer", p.curToken.Literal)
		return nil
	}
	lit.Value = value

	return lit
}

// parseInteger gives an integer literal its value, honouring the 0x, 0b and 0o
// radix prefixes the lexer recognises.
//
// A leading zero with no prefix stays decimal, so `010` is ten. C would read it
// as octal eight, and that rule is a well-known way to lose an afternoon; the
// explicit `0o` is what asks for octal here.
func parseInteger(literal string) (int64, error) {
	base, digits := 10, literal

	if len(literal) > 1 && literal[0] == '0' {
		switch literal[1] {
		case 'x', 'X':
			base, digits = 16, literal[2:]
		case 'b', 'B':
			base, digits = 2, literal[2:]
		case 'o', 'O':
			base, digits = 8, literal[2:]
		}
	}

	return strconv.ParseInt(digits, base, 64)
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}

	var value float64
	if _, err := fmt.Sscanf(p.curToken.Literal, "%g", &value); err != nil {
		p.addError(p.curToken.Pos, "could not parse %q as a number", p.curToken.Literal)
		return nil
	}
	lit.Value = value

	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNullLiteral() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expr := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()
	expr.Right = p.parseExpression(precPrefix)
	if expr.Right == nil {
		return nil
	}

	return expr
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expr := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()

	expr.Right = p.parseExpression(precedence)
	if expr.Right == nil {
		return nil
	}

	return expr
}

// parseAssignExpression parses `name = value`. Assignment is a statement in
// C, but modelling it as an expression keeps the grammar small and allows
// `x = y = 0` as well as assignments inside a for-loop post clause.
func (p *Parser) parseAssignExpression(left ast.Expression) ast.Expression {
	tok := p.curToken

	if _, ok := left.(*ast.Identifier); !ok {
		// a[i] = v is a useful special case.
		if _, isIndex := left.(*ast.IndexExpression); !isIndex {
			p.addError(tok.Pos, "cannot assign to %s: assignment targets must be a name or an array element", left.String())
			return nil
		}
	}

	expr := &ast.InfixExpression{
		Token:    tok,
		Operator: tok.Literal,
		Left:     left,
	}

	// Right-associative, so `a = b = 1` groups as `a = (b = 1)`.
	p.nextToken()
	expr.Right = p.parseExpression(precLowest - 1)
	if expr.Right == nil {
		return nil
	}

	return expr
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	expr := p.parseExpression(precLowest)
	if expr == nil {
		return nil
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return expr
}

func (p *Parser) parseIfExpression() ast.Expression {
	expr := &ast.IfExpression{Token: p.curToken}

	p.nextToken()
	expr.Condition = p.parseExpression(precLowest)
	if expr.Condition == nil {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		switch {
		case p.peekTokenIs(token.IF):
			p.nextToken()
			alt := p.parseIfExpression()
			if alt == nil {
				return nil
			}
			expr.Alternative = alt
		case p.peekTokenIs(token.LBRACE):
			p.nextToken()
			expr.Alternative = p.parseBlockStatement()
		default:
			p.peekError(token.LBRACE)
			return nil
		}
	}

	return expr
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	params, ok := p.parseFunctionParameters()
	if !ok {
		return nil
	}
	lit.Parameters = params

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()

	return lit
}

// parseFunctionParameters parses a parenthesised, comma-separated parameter
// list. curToken must be `(` on entry and is `)` on success.
func (p *Parser) parseFunctionParameters() ([]*ast.Identifier, bool) {
	var params []*ast.Identifier

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params, true
	}

	p.nextToken()

	params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	if !p.expectPeek(token.RPAREN) {
		return nil, false
	}

	return params, true
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	expr := &ast.CallExpression{Token: p.curToken, Function: function}

	args, ok := p.parseExpressionList(token.RPAREN)
	if !ok {
		return nil
	}
	expr.Arguments = args

	return expr
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	arr := &ast.ArrayLiteral{Token: p.curToken}

	elems, ok := p.parseExpressionList(token.RBRACKET)
	if !ok {
		return nil
	}
	arr.Elements = elems

	return arr
}

// parseExpressionList parses a comma-separated list up to (and including) the
// closing token. curToken must be the opening delimiter on entry.
func (p *Parser) parseExpressionList(end token.Type) ([]ast.Expression, bool) {
	var list []ast.Expression

	if p.peekTokenIs(end) {
		p.nextToken()
		return list, true
	}

	p.nextToken()

	first := p.parseExpression(precLowest)
	if first == nil {
		return nil, false
	}
	list = append(list, first)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()

		next := p.parseExpression(precLowest)
		if next == nil {
			return nil, false
		}
		list = append(list, next)
	}

	if !p.expectPeek(end) {
		return nil, false
	}

	return list, true
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	expr := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	expr.Index = p.parseExpression(precLowest)
	if expr.Index == nil {
		return nil
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return expr
}

// ------------------------------------------------------------------- helpers

// describe renders a token for use in an error message.
func describe(t token.Token) string {
	if t.Literal == "" {
		return string(t.Type)
	}
	return fmt.Sprintf("%s %q", t.Type, t.Literal)
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
