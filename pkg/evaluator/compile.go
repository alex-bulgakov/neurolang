package evaluator

import (
	"fmt"
	"neurolang/pkg/ast"
	"neurolang/pkg/object"
)

type compiler struct {
	ch    *chunk
	loops []loopCtx
	hid   int
}

type loopCtx struct {
	breaks    []int
	continues []int
	start     int
	breakPops int
}

func (c *compiler) hidden(prefix string) string {
	c.hid++
	return fmt.Sprintf("%s%d", prefix, c.hid)
}

func compile(node ast.Node) (*chunk, error) {
	c := &compiler{ch: &chunk{}}
	if err := c.compileNode(node, true); err != nil {
		return nil, err
	}
	return c.ch, nil
}

func (c *compiler) compileNode(node ast.Node, keep bool) error {
	if node == nil {
		c.ch.emit(opNull)
		return nil
	}
	switch n := node.(type) {
	case *ast.Program:
		return c.compileProgram(n)
	case *ast.BlockStatement:
		return c.compileBlock(n, keep)
	case *ast.ExpressionStatement:
		return c.compileNode(n.Expression, true)
	case *ast.AssignStatement:
		return c.compileAssign(n)
	case *ast.ReturnStatement:
		if err := c.compileNode(n.ReturnValue, true); err != nil {
			return err
		}
		c.ch.emit(opReturn)
		return nil
	case *ast.WhileStatement:
		return c.compileWhile(n)
	case *ast.ForStatement:
		return c.compileFor(n)
	case *ast.BreakStatement:
		if len(c.loops) == 0 {
			return fmt.Errorf("break outside loop")
		}
		lp := &c.loops[len(c.loops)-1]
		for i := 0; i < lp.breakPops; i++ {
			c.ch.emit(opPop)
		}
		j := c.ch.emitJump(opJump)
		lp.breaks = append(lp.breaks, j)
		return nil
	case *ast.ContinueStatement:
		if len(c.loops) == 0 {
			return fmt.Errorf("continue outside loop")
		}
		j := c.ch.emitJump(opJump)
		c.loops[len(c.loops)-1].continues = append(c.loops[len(c.loops)-1].continues, j)
		return nil
	case *ast.IntegerLiteral:
		c.ch.emit(opConstant, c.ch.addConst(&object.Integer{Value: n.Value}))
		return nil
	case *ast.FloatLiteral:
		c.ch.emit(opConstant, c.ch.addConst(&object.Float{Value: n.Value}))
		return nil
	case *ast.StringLiteral:
		c.ch.emit(opConstant, c.ch.addConst(&object.String{Value: n.Value}))
		return nil
	case *ast.BooleanLiteral:
		if n.Value {
			c.ch.emit(opTrue)
		} else {
			c.ch.emit(opFalse)
		}
		return nil
	case *ast.NullLiteral:
		c.ch.emit(opNull)
		return nil
	case *ast.Identifier:
		c.ch.emit(opGet, c.ch.addName(n.Value))
		return nil
	case *ast.PrefixExpression:
		if err := c.compileNode(n.Right, true); err != nil {
			return err
		}
		if n.Operator == "-" {
			c.ch.emit(opMinus)
		} else {
			c.ch.emit(opBang)
		}
		return nil
	case *ast.InfixExpression:
		return c.compileInfix(n)
	case *ast.ListLiteral:
		for _, el := range n.Elements {
			if err := c.compileNode(el, true); err != nil {
				return err
			}
		}
		c.ch.emit(opArray, len(n.Elements))
		return nil
	case *ast.MapLiteral:
		i := 0
		for k, v := range n.Pairs {
			if err := c.compileNode(k, true); err != nil {
				return err
			}
			if err := c.compileNode(v, true); err != nil {
				return err
			}
			i++
		}
		c.ch.emit(opMap, i)
		return nil
	case *ast.IndexExpression:
		if err := c.compileNode(n.Left, true); err != nil {
			return err
		}
		if err := c.compileNode(n.Index, true); err != nil {
			return err
		}
		c.ch.emit(opIndex)
		return nil
	case *ast.PropertyExpression:
		if err := c.compileNode(n.Left, true); err != nil {
			return err
		}
		c.ch.emit(opProp, c.ch.addName(n.Property))
		return nil
	case *ast.DotExpression:
		if n.Field == "" {
			c.ch.emit(opDot)
		} else {
			c.ch.emit(opDotField, c.ch.addName(n.Field))
		}
		return nil
	case *ast.CallExpression:
		if err := c.compileNode(n.Function, true); err != nil {
			return err
		}
		for _, a := range n.Arguments {
			if err := c.compileNode(a, true); err != nil {
				return err
			}
		}
		c.ch.emit(opCall, len(n.Arguments))
		return nil
	case *ast.ArrowFunctionLiteral:
		return c.compileLambda(n)
	case *ast.IfExpression:
		return c.compileIf(n)
	case *ast.MatchExpression:
		return c.compileMatch(n)
	case *ast.UseExpression:
		if err := c.compileNode(n.Path, true); err != nil {
			return err
		}
		c.ch.emit(opUse)
		return nil
	case *ast.ToolCallExpression:
		for _, a := range n.Arguments {
			if err := c.compileNode(a, true); err != nil {
				return err
			}
		}
		c.ch.emit(opTool, c.ch.addName(n.ToolName), len(n.Arguments))
		return nil
	case *ast.PipeExpression:
		return c.compilePipe(n)
	case *ast.FilterExpression, *ast.MapExpression, *ast.ReduceExpression:
		c.ch.emit(opDot)
		return c.compilePipeRHS(n)
	default:
		return fmt.Errorf("compile: unsupported node %T", node)
	}
}

func (c *compiler) compileProgram(p *ast.Program) error {
	if len(p.Statements) == 0 {
		c.ch.emit(opNull)
		return nil
	}
	for i, s := range p.Statements {
		keep := i == len(p.Statements)-1
		if err := c.compileNode(s, true); err != nil {
			return err
		}
		if !keep {
			c.ch.emit(opPop)
		}
	}
	return nil
}

func (c *compiler) compileBlock(b *ast.BlockStatement, keep bool) error {
	if len(b.Statements) == 0 {
		if keep {
			c.ch.emit(opNull)
		}
		return nil
	}
	for i, s := range b.Statements {
		k := keep && i == len(b.Statements)-1
		if err := c.compileNode(s, true); err != nil {
			return err
		}
		if !k {
			c.ch.emit(opPop)
		}
	}
	return nil
}

func (c *compiler) compileAssign(n *ast.AssignStatement) error {
	if err := c.compileNode(n.Value, true); err != nil {
		return err
	}
	target := n.Target
	if target == nil && n.Name != nil {
		target = n.Name
	}
	switch t := target.(type) {
	case *ast.Identifier:
		c.ch.emit(opSet, c.ch.addName(t.Value))
	case *ast.PropertyExpression:
		if err := c.compileNode(t.Left, true); err != nil {
			return err
		}
		c.ch.emit(opSetProp, c.ch.addName(t.Property))
	case *ast.IndexExpression:
		if err := c.compileNode(t.Left, true); err != nil {
			return err
		}
		if err := c.compileNode(t.Index, true); err != nil {
			return err
		}
		c.ch.emit(opSetIndex)
	default:
		return fmt.Errorf("compile: bad assign target %T", target)
	}
	return nil
}

func (c *compiler) compileInfix(n *ast.InfixExpression) error {
	switch n.Operator {
	case "&&":
		if err := c.compileNode(n.Left, true); err != nil {
			return err
		}
		jfalse := c.ch.emitJump(opJumpFalse)
		c.ch.emit(opPop)
		if err := c.compileNode(n.Right, true); err != nil {
			return err
		}
		c.ch.emit(opToBool)
		jend := c.ch.emitJump(opJump)
		c.ch.patch(jfalse, len(c.ch.code))
		c.ch.emit(opPop)
		c.ch.emit(opFalse)
		c.ch.patch(jend, len(c.ch.code))
		return nil
	case "||":
		if err := c.compileNode(n.Left, true); err != nil {
			return err
		}
		jtrue := c.ch.emitJump(opJumpTrue)
		c.ch.emit(opPop)
		if err := c.compileNode(n.Right, true); err != nil {
			return err
		}
		c.ch.emit(opToBool)
		jend := c.ch.emitJump(opJump)
		c.ch.patch(jtrue, len(c.ch.code))
		c.ch.emit(opPop)
		c.ch.emit(opTrue)
		c.ch.patch(jend, len(c.ch.code))
		return nil
	case "??":
		if err := c.compileNode(n.Left, true); err != nil {
			return err
		}
		jok := c.ch.emitJump(opJumpOk)
		c.ch.emit(opPop)
		if err := c.compileNode(n.Right, true); err != nil {
			return err
		}
		c.ch.patch(jok, len(c.ch.code))
		return nil
	}
	if err := c.compileNode(n.Left, true); err != nil {
		return err
	}
	if err := c.compileNode(n.Right, true); err != nil {
		return err
	}
	ops := map[string]opcode{
		"+": opAdd, "-": opSub, "*": opMul, "/": opDiv, "%": opMod,
		"==": opEq, "!=": opNeq, "<": opLt, "<=": opLte, ">": opGt, ">=": opGte, "in": opIn,
	}
	op, ok := ops[n.Operator]
	if !ok {
		return fmt.Errorf("compile: unknown operator %s", n.Operator)
	}
	c.ch.emit(op)
	return nil
}

func (c *compiler) compileIf(n *ast.IfExpression) error {
	if err := c.compileNode(n.Condition, true); err != nil {
		return err
	}
	jfalse := c.ch.emitJump(opJumpFalse)
	c.ch.emit(opPop)
	if err := c.compileNode(n.Consequence, true); err != nil {
		return err
	}
	jend := c.ch.emitJump(opJump)
	c.ch.patch(jfalse, len(c.ch.code))
	c.ch.emit(opPop)
	if n.Alternative != nil {
		if err := c.compileNode(n.Alternative, true); err != nil {
			return err
		}
	} else {
		c.ch.emit(opNull)
	}
	c.ch.patch(jend, len(c.ch.code))
	return nil
}

func (c *compiler) compileWhile(n *ast.WhileStatement) error {
	start := len(c.ch.code)
	c.loops = append(c.loops, loopCtx{start: start})
	if err := c.compileNode(n.Condition, true); err != nil {
		return err
	}
	jfalse := c.ch.emitJump(opJumpFalse)
	c.ch.emit(opPop)
	if err := c.compileNode(n.Body, false); err != nil {
		return err
	}
	c.ch.emit(opJump, start)
	fail := len(c.ch.code)
	c.ch.patch(jfalse, fail)
	c.ch.emit(opPop)
	end := len(c.ch.code)
	c.ch.emit(opNull)
	loop := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]
	for _, b := range loop.breaks {
		c.ch.patch(b, end)
	}
	for _, ct := range loop.continues {
		c.ch.patch(ct, start)
	}
	return nil
}

func (c *compiler) compileFor(n *ast.ForStatement) error {
	if err := c.compileNode(n.Iterable, true); err != nil {
		return err
	}
	c.ch.emit(opIter)
	start := len(c.ch.code)
	c.loops = append(c.loops, loopCtx{start: start, breakPops: 1})
	jdone := c.ch.emitJump(opIterNext)
	c.ch.emit(opSetDot)
	c.ch.emit(opSet, c.ch.addName(n.Name.Value))
	c.ch.emit(opPop)
	if err := c.compileNode(n.Body, false); err != nil {
		return err
	}
	c.ch.emit(opJump, start)
	end := len(c.ch.code)
	c.ch.patch(jdone, end)
	c.ch.emit(opNull)
	loop := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]
	for _, b := range loop.breaks {
		c.ch.patch(b, end)
	}
	for _, ct := range loop.continues {
		c.ch.patch(ct, start)
	}
	return nil
}

func (c *compiler) compileLambda(n *ast.ArrowFunctionLiteral) error {
	inner := &compiler{ch: &chunk{}}
	if err := inner.compileNode(n.Body, true); err != nil {
		return err
	}
	inner.ch.emit(opReturn)
	params := make([]string, len(n.Parameters))
	for i, p := range n.Parameters {
		params[i] = p.Value
	}
	cl := &vmClosure{ch: inner.ch, params: params}
	c.ch.emit(opClosure, c.ch.addConst(cl))
	return nil
}

func (c *compiler) compileMatch(n *ast.MatchExpression) error {
	hid := c.hidden("__m")
	if err := c.compileNode(n.Subject, true); err != nil {
		return err
	}
	c.ch.emit(opSet, c.ch.addName(hid))
	c.ch.emit(opPop)
	var ends []int
	for _, cs := range n.Cases {
		if ident, ok := cs.Pattern.(*ast.Identifier); ok && ident.Value == "_" {
			if err := c.compileNode(cs.Body, true); err != nil {
				return err
			}
			ends = append(ends, c.ch.emitJump(opJump))
			continue
		}
		if err := c.compileNode(cs.Pattern, true); err != nil {
			return err
		}
		c.ch.emit(opGet, c.ch.addName(hid))
		c.ch.emit(opEq)
		jfalse := c.ch.emitJump(opJumpFalse)
		c.ch.emit(opPop)
		if err := c.compileNode(cs.Body, true); err != nil {
			return err
		}
		ends = append(ends, c.ch.emitJump(opJump))
		c.ch.patch(jfalse, len(c.ch.code))
		c.ch.emit(opPop)
	}
	c.ch.emit(opNull)
	end := len(c.ch.code)
	for _, e := range ends {
		c.ch.patch(e, end)
	}
	return nil
}

func (c *compiler) compilePipe(n *ast.PipeExpression) error {
	if err := c.compileNode(n.Left, true); err != nil {
		return err
	}
	return c.compilePipeRHS(n.Right)
}

func (c *compiler) compilePipeRHS(right ast.Node) error {
	switch r := right.(type) {
	case *ast.FilterExpression:
		inner := &compiler{ch: &chunk{}}
		if err := inner.compileNode(r.Predicate, true); err != nil {
			return err
		}
		inner.ch.emit(opReturn)
		c.ch.emit(opClosure, c.ch.addConst(&vmClosure{ch: inner.ch}))
		c.ch.emit(opPipeFilter)
		return nil
	case *ast.MapExpression:
		inner := &compiler{ch: &chunk{}}
		if err := inner.compileNode(r.Transform, true); err != nil {
			return err
		}
		inner.ch.emit(opReturn)
		c.ch.emit(opClosure, c.ch.addConst(&vmClosure{ch: inner.ch}))
		c.ch.emit(opPipeMap)
		return nil
	case *ast.ReduceExpression:
		if err := c.compileNode(r.Reducer, true); err != nil {
			return err
		}
		c.ch.emit(opPipeReduce)
		return nil
	case *ast.ToolCallExpression:
		for _, a := range r.Arguments {
			if err := c.compileNode(a, true); err != nil {
				return err
			}
		}
		c.ch.emit(opTool, c.ch.addName(r.ToolName), len(r.Arguments)+1)
		return nil
	case *ast.CallExpression:
		if err := c.compileNode(r.Function, true); err != nil {
			return err
		}
		// stack: left, fn  — need fn, left, args. Swap via pipe-call op that prepends.
		for _, a := range r.Arguments {
			if err := c.compileNode(a, true); err != nil {
				return err
			}
		}
		c.ch.emit(opPipeCall, len(r.Arguments)+1)
		return nil
	default:
		if err := c.compileNode(right, true); err != nil {
			return err
		}
		c.ch.emit(opPipeCall, 1)
		return nil
	}
}
