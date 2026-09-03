package ast

import (
	"bytes"
	"fmt"
	"neurolang/pkg/token"
	"strings"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

// Program is the root of every AST
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String() + "\n")
	}
	return out.String()
}

// Statements

type AssignStatement struct {
	Token  token.Token // '=' token
	Name   *Identifier // for backward compatibility when Target is *Identifier
	Target Expression  // Identifier, PropertyExpression, or IndexExpression
	Value  Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) String() string {
	targetStr := ""
	if as.Target != nil {
		targetStr = as.Target.String()
	} else if as.Name != nil {
		targetStr = as.Name.String()
	}
	return fmt.Sprintf("%s = %s", targetStr, as.Value.String())
}

type ExpressionStatement struct {
	Token      token.Token // First token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

type BlockStatement struct {
	Token      token.Token // '{'
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) expressionNode()      {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	out.WriteString("{\n")
	for _, s := range bs.Statements {
		out.WriteString("  " + s.String() + "\n")
	}
	out.WriteString("}")
	return out.String()
}

type ReturnStatement struct {
	Token       token.Token // 'return'
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	if rs.ReturnValue != nil {
		return "return " + rs.ReturnValue.String()
	}
	return "return"
}

type WhileStatement struct {
	Token     token.Token // 'while'
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	return "while " + ws.Condition.String() + " " + ws.Body.String()
}

type ForStatement struct {
	Token    token.Token // 'for'
	Name     *Identifier
	Iterable Expression
	Body     *BlockStatement
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) String() string {
	return "for " + fs.Name.String() + " in " + fs.Iterable.String() + " " + fs.Body.String()
}

type BreakStatement struct {
	Token token.Token // 'break'
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) String() string       { return "break" }

type ContinueStatement struct {
	Token token.Token // 'continue'
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) String() string       { return "continue" }

// Expressions

type Identifier struct {
	Token token.Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return fmt.Sprintf("%q", sl.Value) }

type BooleanLiteral struct {
	Token token.Token
	Value bool
}

func (b *BooleanLiteral) expressionNode()      {}
func (b *BooleanLiteral) TokenLiteral() string { return b.Token.Literal }
func (b *BooleanLiteral) String() string       { return b.Token.Literal }

type NullLiteral struct {
	Token token.Token
}

func (n *NullLiteral) expressionNode()      {}
func (n *NullLiteral) TokenLiteral() string { return "null" }
func (n *NullLiteral) String() string       { return "null" }

type ListLiteral struct {
	Token    token.Token // '['
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }
func (ll *ListLiteral) String() string {
	elements := make([]string, len(ll.Elements))
	for i, el := range ll.Elements {
		elements[i] = el.String()
	}
	return "[" + strings.Join(elements, ", ") + "]"
}

type MapLiteral struct {
	Token token.Token // '{'
	Pairs map[Expression]Expression
}

func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) String() string {
	var pairs []string
	for k, v := range ml.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", k.String(), v.String()))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}

// DotExpression represents current element or property navigation, e.g. '.', '.age', '.user.name'
type DotExpression struct {
	Token token.Token // '.'
	Field string      // empty string if just '.' (the entire item itself)
}

func (de *DotExpression) expressionNode()      {}
func (de *DotExpression) TokenLiteral() string { return de.Token.Literal }
func (de *DotExpression) String() string {
	if de.Field == "" {
		return "."
	}
	return "." + de.Field
}

type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	return "(" + pe.Operator + pe.Right.String() + ")"
}

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (oe *InfixExpression) expressionNode()      {}
func (oe *InfixExpression) TokenLiteral() string { return oe.Token.Literal }
func (oe *InfixExpression) String() string {
	return "(" + oe.Left.String() + " " + oe.Operator + " " + oe.Right.String() + ")"
}

// AI-native Dataflow Expressions:

// PipeExpression: left | right
type PipeExpression struct {
	Token token.Token // '|'
	Left  Expression
	Right Expression
}

func (pe *PipeExpression) expressionNode()      {}
func (pe *PipeExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PipeExpression) String() string {
	return pe.Left.String() + " | " + pe.Right.String()
}

// FilterExpression: ?(condition) or ?pred
type FilterExpression struct {
	Token     token.Token // '?'
	Predicate Expression
}

func (fe *FilterExpression) expressionNode()      {}
func (fe *FilterExpression) TokenLiteral() string { return fe.Token.Literal }
func (fe *FilterExpression) String() string {
	return "?" + fe.Predicate.String()
}

// MapExpression: @expr or @.field
type MapExpression struct {
	Token     token.Token // '@'
	Transform Expression
}

func (me *MapExpression) expressionNode()      {}
func (me *MapExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MapExpression) String() string {
	return "@" + me.Transform.String()
}

// ReduceExpression: &expr
type ReduceExpression struct {
	Token   token.Token // '&'
	Reducer Expression
}

func (re *ReduceExpression) expressionNode()      {}
func (re *ReduceExpression) TokenLiteral() string { return re.Token.Literal }
func (re *ReduceExpression) String() string {
	return "&" + re.Reducer.String()
}

// ToolCallExpression: !tool.method(args...) or !ident(args...)
type ToolCallExpression struct {
	Token     token.Token // '!'
	ToolName  string
	Arguments []Expression
}

func (tce *ToolCallExpression) expressionNode()      {}
func (tce *ToolCallExpression) TokenLiteral() string { return tce.Token.Literal }
func (tce *ToolCallExpression) String() string {
	args := make([]string, len(tce.Arguments))
	for i, arg := range tce.Arguments {
		args[i] = arg.String()
	}
	return "!" + tce.ToolName + "(" + strings.Join(args, ", ") + ")"
}

type CallExpression struct {
	Token     token.Token // '('
	Function  Expression  // Identifier or Expression
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	args := make([]string, len(ce.Arguments))
	for i, a := range ce.Arguments {
		args[i] = a.String()
	}
	return ce.Function.String() + "(" + strings.Join(args, ", ") + ")"
}

type IndexExpression struct {
	Token token.Token // '['
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	return "(" + ie.Left.String() + "[" + ie.Index.String() + "])"
}

type PropertyExpression struct {
	Token    token.Token // '.'
	Left     Expression
	Property string
}

func (pe *PropertyExpression) expressionNode()      {}
func (pe *PropertyExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PropertyExpression) String() string {
	return pe.Left.String() + "." + pe.Property
}

type ArrowFunctionLiteral struct {
	Token      token.Token // '->'
	Parameters []*Identifier
	Body       Expression
}

func (af *ArrowFunctionLiteral) expressionNode()      {}
func (af *ArrowFunctionLiteral) TokenLiteral() string { return af.Token.Literal }
func (af *ArrowFunctionLiteral) String() string {
	params := make([]string, len(af.Parameters))
	for i, p := range af.Parameters {
		params[i] = p.String()
	}
	return "(" + strings.Join(params, ", ") + ") -> " + af.Body.String()
}

type IfExpression struct {
	Token       token.Token // 'if' or '?'
	Condition   Expression
	Consequence Expression
	Alternative Expression
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	var out bytes.Buffer
	out.WriteString("if " + ie.Condition.String() + " " + ie.Consequence.String())
	if ie.Alternative != nil {
		out.WriteString(" else " + ie.Alternative.String())
	}
	return out.String()
}

type UseExpression struct {
	Token token.Token // 'use'
	Path  Expression
}

func (ue *UseExpression) expressionNode()      {}
func (ue *UseExpression) TokenLiteral() string { return ue.Token.Literal }
func (ue *UseExpression) String() string {
	if ue.Path != nil {
		return "use " + ue.Path.String()
	}
	return "use"
}

type MatchCase struct {
	Pattern Expression // Literal or Identifier (e.g. `_` for default)
	Body    Expression
}

type MatchExpression struct {
	Token   token.Token // 'match'
	Subject Expression
	Cases   []MatchCase
}

func (me *MatchExpression) expressionNode()      {}
func (me *MatchExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MatchExpression) String() string {
	var out bytes.Buffer
	out.WriteString("match " + me.Subject.String() + " {\n")
	for _, c := range me.Cases {
		out.WriteString(fmt.Sprintf("  %s -> %s\n", c.Pattern.String(), c.Body.String()))
	}
	out.WriteString("}")
	return out.String()
}
