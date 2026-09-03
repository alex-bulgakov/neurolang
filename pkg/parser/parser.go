package parser

import (
	"fmt"
	"neurolang/pkg/ast"
	"neurolang/pkg/lexer"
	"neurolang/pkg/token"
	"strconv"
)

const (
	_ int = iota
	LOWEST
	PIPE        // |
	LOGICAL_OR  // ||
	LOGICAL_AND // &&
	EQUALS      // ==, !=
	LESSGREATER // >, <, >=, <=
	SUM         // +, -
	PRODUCT     // *, /, %
	PREFIX      // -X, !X, ?X, @X, &X
	CALL        // myFunction(X)
	INDEX       // array[index], obj.property
)

var precedences = map[token.TokenType]int{
	token.PIPE:      PIPE,
	token.OR:        LOGICAL_OR,
	token.AND:       LOGICAL_AND,
	token.EQ:        EQUALS,
	token.NOT_EQ:    EQUALS,
	token.LT:        LESSGREATER,
	token.LTE:       LESSGREATER,
	token.GT:        LESSGREATER,
	token.GTE:       LESSGREATER,
	token.PLUS:      SUM,
	token.MINUS:     SUM,
	token.SLASH:     PRODUCT,
	token.ASTERISK:  PRODUCT,
	token.MOD:       PRODUCT,
	token.LPAREN:    CALL,
	token.LBRACKET:  INDEX,
	token.DOT:       INDEX,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifierOrArrow)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(token.FALSE, p.parseBooleanLiteral)
	p.registerPrefix(token.NULL, p.parseNullLiteral)
	p.registerPrefix(token.BANG, p.parseBangOrToolCall)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedOrArrowFunction)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.LBRACE, p.parseMapOrBlock)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.MATCH, p.parseMatchExpression)

	// AI Combinators as prefix
	p.registerPrefix(token.FILTER, p.parseFilterExpression)
	p.registerPrefix(token.MAP, p.parseMapExpression)
	p.registerPrefix(token.DOT, p.parseDotExpression)
	p.registerPrefix(token.REDUCE, p.parseReduceExpression)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.MOD, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.PIPE, p.parsePipeExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parsePropertyExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("line %d, col %d: expected next token to be %s, got %s instead (%q)",
		p.peekToken.Line, p.peekToken.Col, t, p.peekToken.Type, p.peekToken.Literal)
	p.errors = append(p.errors, msg)
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{
		Statements: []ast.Statement{},
	}

	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	// Assignment: ident = expr
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
		return p.parseAssignStatement()
	}

	if p.curTokenIs(token.RETURN) {
		return p.parseReturnStatement()
	}

	if p.curTokenIs(token.WHILE) {
		return p.parseWhileStatement()
	}

	if p.curTokenIs(token.BREAK) {
		stmt := &ast.BreakStatement{Token: p.curToken}
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		return stmt
	}

	if p.curTokenIs(token.CONTINUE) {
		stmt := &ast.ContinueStatement{Token: p.curToken}
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		return stmt
	}

	return p.parseExpressionStatement()
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	stmt := &ast.WhileStatement{Token: p.curToken}
	p.nextToken() // move past 'while'

	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseAssignStatement() *ast.AssignStatement {
	stmt := &ast.AssignStatement{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	p.nextToken() // move to '='
	p.nextToken() // move to expression start

	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}
	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		// If peekToken is on a new line, only explicit continuation operators can extend the expression
		if p.peekToken.Line > p.curToken.Line {
			if p.peekToken.Type != token.PIPE &&
				p.peekToken.Type != token.AND &&
				p.peekToken.Type != token.OR &&
				p.peekToken.Type != token.PLUS &&
				p.peekToken.Type != token.MINUS &&
				p.peekToken.Type != token.SLASH &&
				p.peekToken.Type != token.ASTERISK {
				break
			}
		}

		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("line %d, col %d: no prefix parse function for %s (%q)",
		p.curToken.Line, p.curToken.Col, t, p.curToken.Literal)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseIdentifierOrArrow() ast.Expression {
	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Single-parameter arrow function: x -> expr
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // move to '->'
		tok := p.curToken
		p.nextToken() // move to body
		body := p.parseExpression(LOWEST)
		return &ast.ArrowFunctionLiteral{
			Token:      tok,
			Parameters: []*ast.Identifier{ident},
			Body:       body,
		}
	}

	return ident
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}
	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}
	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.curToken.Literal)
		p.errors = append(p.errors, msg)
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
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
}

func (p *Parser) parseBangOrToolCall() ast.Expression {
	bangToken := p.curToken

	// Check if this is a tool call: !tool_name(...) or !module.method(...)
	if p.peekTokenIs(token.IDENT) {
		p.nextToken() // move to tool name
		toolName := p.curToken.Literal

		// Support nested tool names like !http.get(...)
		for p.peekTokenIs(token.DOT) {
			p.nextToken() // consume '.'
			if p.expectPeek(token.IDENT) {
				toolName += "." + p.curToken.Literal
			}
		}

		if p.peekTokenIs(token.LPAREN) {
			p.nextToken() // move to '('
			args := p.parseExpressionList(token.RPAREN)
			return &ast.ToolCallExpression{
				Token:     bangToken,
				ToolName:  toolName,
				Arguments: args,
			}
		}

		// If no '(', treat as tool call with no args: !tool
		return &ast.ToolCallExpression{
			Token:     bangToken,
			ToolName:  toolName,
			Arguments: []ast.Expression{},
		}
	}

	// Otherwise, it's boolean NOT
	p.nextToken()
	right := p.parseExpression(PREFIX)
	return &ast.PrefixExpression{
		Token:    bangToken,
		Operator: "!",
		Right:    right,
	}
}

func (p *Parser) parseDotExpression() ast.Expression {
	dotTok := p.curToken

	// Check if followed by identifier, e.g. .age or .user.name
	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		field := p.curToken.Literal

		// Chained dots: .user.profile.name
		for p.peekTokenIs(token.DOT) {
			p.nextToken() // move to '.'
			if p.peekTokenIs(token.IDENT) {
				p.nextToken()
				field += "." + p.curToken.Literal
			}
		}

		return &ast.DotExpression{
			Token: dotTok,
			Field: field,
		}
	}

	// Just solitary '.', representing the current item itself
	return &ast.DotExpression{
		Token: dotTok,
		Field: "",
	}
}

func (p *Parser) parseFilterExpression() ast.Expression {
	tok := p.curToken
	p.nextToken()
	pred := p.parseExpression(PREFIX)
	return &ast.FilterExpression{
		Token:     tok,
		Predicate: pred,
	}
}

func (p *Parser) parseMapExpression() ast.Expression {
	tok := p.curToken
	p.nextToken()
	transform := p.parseExpression(PREFIX)
	return &ast.MapExpression{
		Token:     tok,
		Transform: transform,
	}
}

func (p *Parser) parseReduceExpression() ast.Expression {
	tok := p.curToken
	p.nextToken()
	reducer := p.parseExpression(PREFIX)
	return &ast.ReduceExpression{
		Token:   tok,
		Reducer: reducer,
	}
}

func (p *Parser) parseGroupedOrArrowFunction() ast.Expression {
	lparen := p.curToken

	// Check for empty params arrow function: () -> expr
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // move to ')'
		if p.peekTokenIs(token.ARROW) {
			p.nextToken() // move to '->'
			arrowTok := p.curToken
			p.nextToken() // move to body
			body := p.parseExpression(LOWEST)
			return &ast.ArrowFunctionLiteral{
				Token:      arrowTok,
				Parameters: []*ast.Identifier{},
				Body:       body,
			}
		}
		// Empty parens not followed by arrow is null/void
		return &ast.NullLiteral{Token: lparen}
	}

	p.nextToken() // move past '('

	// Parse first expression inside parens
	firstExp := p.parseExpression(LOWEST)

	// Check if this is a comma-separated parameter list for an arrow function: (a, b) -> ...
	if p.curTokenIs(token.COMMA) || p.peekTokenIs(token.COMMA) {
		params := []*ast.Identifier{}
		if ident, ok := firstExp.(*ast.Identifier); ok {
			params = append(params, ident)
		}

		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume ','
			p.nextToken() // move to next param
			if ident, ok := p.parseIdentifierOrArrow().(*ast.Identifier); ok {
				params = append(params, ident)
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}

		if p.expectPeek(token.ARROW) {
			arrowTok := p.curToken
			p.nextToken()
			body := p.parseExpression(LOWEST)
			return &ast.ArrowFunctionLiteral{
				Token:      arrowTok,
				Parameters: params,
				Body:       body,
			}
		}
		return nil
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	// Check if single param in parens: (x) -> expr
	if p.peekTokenIs(token.ARROW) {
		if ident, ok := firstExp.(*ast.Identifier); ok {
			p.nextToken() // move to '->'
			arrowTok := p.curToken
			p.nextToken() // move to body
			body := p.parseExpression(LOWEST)
			return &ast.ArrowFunctionLiteral{
				Token:      arrowTok,
				Parameters: []*ast.Identifier{ident},
				Body:       body,
			}
		}
	}

	return firstExp
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parsePipeExpression(left ast.Expression) ast.Expression {
	pipeTok := p.curToken
	p.nextToken()
	right := p.parseExpression(PIPE)
	return &ast.PipeExpression{
		Token: pipeTok,
		Left:  left,
		Right: right,
	}
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parsePropertyExpression(left ast.Expression) ast.Expression {
	dotTok := p.curToken
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	return &ast.PropertyExpression{
		Token:    dotTok,
		Left:     left,
		Property: p.curToken.Literal,
	}
}

func (p *Parser) parseListLiteral() ast.Expression {
	list := &ast.ListLiteral{Token: p.curToken}
	list.Elements = p.parseExpressionList(token.RBRACKET)
	return list
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseMapOrBlock() ast.Expression {
	braceTok := p.curToken

	// Empty map {}
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return &ast.MapLiteral{
			Token: braceTok,
			Pairs: make(map[ast.Expression]ast.Expression),
		}
	}

	p.nextToken() // move past '{'

	// If current token is followed by ':', it's a MapLiteral: { key: val, ... }
	if (p.curTokenIs(token.IDENT) || p.curTokenIs(token.STRING)) && p.peekTokenIs(token.COLON) {
		pairs := make(map[ast.Expression]ast.Expression)

		for {
			key := p.parseMapKey()
			if !p.expectPeek(token.COLON) {
				return nil
			}
			p.nextToken()
			val := p.parseExpression(LOWEST)
			pairs[key] = val

			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume ','
				if p.peekTokenIs(token.RBRACE) {
					break
				}
				p.nextToken() // move to next key
				continue
			}
			break
		}

		if !p.expectPeek(token.RBRACE) {
			return nil
		}

		return &ast.MapLiteral{
			Token: braceTok,
			Pairs: pairs,
		}
	}

	// Otherwise, it is a BlockStatement: { stmt; stmt; ... }
	statements := []ast.Statement{}
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
		p.nextToken()
	}

	return &ast.BlockStatement{
		Token:      braceTok,
		Statements: statements,
	}
}

func (p *Parser) parseMapKey() ast.Expression {
	// Support unquoted keys `{name: "Alice"}` as string literals or identifiers
	if p.curTokenIs(token.IDENT) {
		return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	}
	return p.parseExpression(LOWEST)
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if p.peekTokenIs(token.IF) {
			p.nextToken()
			expression.Alternative = p.parseIfExpression()
		} else if p.expectPeek(token.LBRACE) {
			expression.Alternative = p.parseBlockStatement()
		}
	}

	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken, Statements: []ast.Statement{}}
	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseMatchExpression() ast.Expression {
	tok := p.curToken
	p.nextToken()

	subject := p.parseExpression(LOWEST)

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	matchExpr := &ast.MatchExpression{
		Token:   tok,
		Subject: subject,
		Cases:   []ast.MatchCase{},
	}

	p.nextToken() // move into braces

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		pattern := p.parsePattern()
		if !p.expectPeek(token.ARROW) {
			return nil
		}
		p.nextToken()
		body := p.parseExpression(LOWEST)

		matchExpr.Cases = append(matchExpr.Cases, ast.MatchCase{
			Pattern: pattern,
			Body:    body,
		})

		if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken()
	}

	return matchExpr
}

func (p *Parser) parsePattern() ast.Expression {
	switch p.curToken.Type {
	case token.IDENT:
		return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	case token.INT:
		return p.parseIntegerLiteral()
	case token.FLOAT:
		return p.parseFloatLiteral()
	case token.STRING:
		return p.parseStringLiteral()
	case token.TRUE, token.FALSE:
		return p.parseBooleanLiteral()
	case token.NULL:
		return p.parseNullLiteral()
	default:
		return p.parseExpression(LOWEST)
	}
}
