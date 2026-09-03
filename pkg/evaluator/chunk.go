package evaluator

import "neurolang/pkg/object"

type opcode byte

const (
	opConstant opcode = iota
	opTrue
	opFalse
	opNull
	opPop
	opGet
	opSet
	opAdd
	opSub
	opMul
	opDiv
	opMod
	opEq
	opNeq
	opLt
	opLte
	opGt
	opGte
	opIn
	opMinus
	opBang
	opJump
	opJumpFalse
	opJumpTrue
	opJumpOk
	opToBool
	opArray
	opMap
	opIndex
	opSetIndex
	opProp
	opSetProp
	opCall
	opReturn
	opClosure
	opDot
	opDotField
	opPipeFilter
	opPipeMap
	opPipeReduce
	opPipeCall
	opTool
	opUse
	opIter
	opIterNext
	opSetDot
	opBreak
	opContinue
)

type chunk struct {
	code   []byte
	consts []object.Object
	names  []string
}

var opcodeNames = []string{
	"constant", "true", "false", "null", "pop", "get", "set",
	"add", "sub", "mul", "div", "mod", "eq", "neq", "lt", "lte", "gt", "gte", "in",
	"minus", "bang", "jump", "jump_false", "jump_true", "jump_ok", "to_bool",
	"array", "map", "index", "set_index", "prop", "set_prop",
	"call", "return", "closure", "dot", "dot_field",
	"pipe_filter", "pipe_map", "pipe_reduce", "pipe_call",
	"tool", "use", "iter", "iter_next", "set_dot", "break", "continue",
}

type vmClosure struct {
	ch     *chunk
	params []string
	env    *object.Environment
}

func (c *vmClosure) Type() object.ObjectType { return object.FUNCTION_OBJ }
func (c *vmClosure) Inspect() string         { return "<compiled>" }
func (c *vmClosure) ToInterface() any        { return "<compiled>" }

func (ch *chunk) emit(op opcode, operands ...int) int {
	pos := len(ch.code)
	ch.code = append(ch.code, byte(op))
	for _, o := range operands {
		ch.code = append(ch.code, byte(o>>8), byte(o))
	}
	return pos
}

func (ch *chunk) emitJump(op opcode) int {
	return ch.emit(op, 0)
}

func (ch *chunk) patch(pos, target int) {
	ch.code[pos+1] = byte(target >> 8)
	ch.code[pos+2] = byte(target)
}

func (ch *chunk) addConst(v object.Object) int {
	ch.consts = append(ch.consts, v)
	return len(ch.consts) - 1
}

func (ch *chunk) addName(n string) int {
	for i, s := range ch.names {
		if s == n {
			return i
		}
	}
	ch.names = append(ch.names, n)
	return len(ch.names) - 1
}

func readU16(code []byte, ip int) int {
	return int(code[ip])<<8 | int(code[ip+1])
}

func init() {
	if len(opcodeNames) != int(opContinue)+1 {
		panic("opcodeNames out of sync with opcodes")
	}
}
