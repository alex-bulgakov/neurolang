package lexer

import (
	"neurolang/pkg/token"
	"testing"
)

func TestNextToken(t *testing.T) {
	input := `
# Simple assignment
x = 42
pi = 3.14
msg = "hello, world!"

# Pipeline with filter and map
$users | ?(.age >= 18) | @.name | !print

# Arrow function and reduce
sum = (a, b) -> a + b
numbers = [1, 2, 3]
total = numbers | &(sum)

# Match
res = match x {
    42 -> "answer"
    _ -> "other"
}
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "42"},

		{token.IDENT, "pi"},
		{token.ASSIGN, "="},
		{token.FLOAT, "3.14"},

		{token.IDENT, "msg"},
		{token.ASSIGN, "="},
		{token.STRING, "hello, world!"},

		// Pipeline
		{token.IDENT, "$users"},
		{token.PIPE, "|"},
		{token.FILTER, "?"},
		{token.LPAREN, "("},
		{token.DOT, "."},
		{token.IDENT, "age"},
		{token.GTE, ">="},
		{token.INT, "18"},
		{token.RPAREN, ")"},
		{token.PIPE, "|"},
		{token.MAP, "@"},
		{token.DOT, "."},
		{token.IDENT, "name"},
		{token.PIPE, "|"},
		{token.BANG, "!"},
		{token.IDENT, "print"},

		// Arrow function
		{token.IDENT, "sum"},
		{token.ASSIGN, "="},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.RPAREN, ")"},
		{token.ARROW, "->"},
		{token.IDENT, "a"},
		{token.PLUS, "+"},
		{token.IDENT, "b"},

		// List
		{token.IDENT, "numbers"},
		{token.ASSIGN, "="},
		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.COMMA, ","},
		{token.INT, "3"},
		{token.RBRACKET, "]"},

		// Reduce
		{token.IDENT, "total"},
		{token.ASSIGN, "="},
		{token.IDENT, "numbers"},
		{token.PIPE, "|"},
		{token.REDUCE, "&"},
		{token.LPAREN, "("},
		{token.IDENT, "sum"},
		{token.RPAREN, ")"},

		// Match
		{token.IDENT, "res"},
		{token.ASSIGN, "="},
		{token.MATCH, "match"},
		{token.IDENT, "x"},
		{token.LBRACE, "{"},
		{token.INT, "42"},
		{token.ARROW, "->"},
		{token.STRING, "answer"},
		{token.IDENT, "_"},
		{token.ARROW, "->"},
		{token.STRING, "other"},
		{token.RBRACE, "}"},

		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
