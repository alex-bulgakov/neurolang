package token

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"
	COMMENT = "COMMENT"

	// Identifiers and Literals
	IDENT  = "IDENT"
	INT    = "INT"
	FLOAT  = "FLOAT"
	STRING = "STRING"

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!" // Tool effect call or logical NOT
	ASTERISK = "*" // Multiply or flatMap
	SLASH    = "/"
	MOD      = "%"

	EQ     = "=="
	NOT_EQ = "!="
	LT     = "<"
	LTE    = "<="
	GT     = ">"
	GTE    = ">="

	AND = "&&"
	OR  = "||"

	// Dataflow & AI Combinators
	PIPE   = "|"  // Dataflow pipeline
	FILTER = "?"  // Filter stream / condition guard
	MAP    = "@"  // Projection / map
	DOT    = "."  // Current item / field accessor
	REDUCE = "&"  // Fold / reduce
	ARROW  = "->" // Lambda arrow

	// Delimiters
	COMMA     = ","
	COLON     = ":"
	SEMICOLON = ";"

	LPAREN   = "("
	RPAREN   = ")"
	LBRACKET = "["
	RBRACKET = "]"
	LBRACE   = "{"
	RBRACE   = "}"

	// Keywords
	TRUE   = "true"
	FALSE  = "false"
	NULL   = "null"
	IF     = "if"
	ELSE   = "else"
	MATCH  = "match"
	FN     = "fn"
	RETURN = "return"
)

var keywords = map[string]TokenType{
	"true":   TRUE,
	"false":  FALSE,
	"null":   NULL,
	"if":     IF,
	"else":   ELSE,
	"match":  MATCH,
	"fn":     FN,
	"return": RETURN,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
