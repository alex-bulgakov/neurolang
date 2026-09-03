package evaluator

import (
	"neurolang/pkg/object"
	"strings"
)

type iterator struct {
	items []object.Object
	i     int
}

func (it *iterator) Type() object.ObjectType { return "ITER" }
func (it *iterator) Inspect() string         { return "<iter>" }
func (it *iterator) ToInterface() any        { return nil }

type machine struct {
	ch   *chunk
	ip   int
	stack []object.Object
	env  *object.Environment
}

func run(ch *chunk, env *object.Environment) object.Object {
	m := &machine{ch: ch, env: env}
	return m.exec(ch, env)
}

func (m *machine) exec(ch *chunk, env *object.Environment) object.Object {
	ip := 0
	code := ch.code
	for ip < len(code) {
		op := opcode(code[ip])
		ip++
		switch op {
		case opConstant:
			idx := readU16(code, ip)
			ip += 2
			m.push(ch.consts[idx])
		case opTrue:
			m.push(TRUE)
		case opFalse:
			m.push(FALSE)
		case opNull:
			m.push(NULL)
		case opPop:
			m.pop()
		case opGet:
			idx := readU16(code, ip)
			ip += 2
			name := ch.names[idx]
			if val, ok := env.Get(name); ok {
				m.push(val)
			} else if b, ok := builtins[name]; ok {
				m.push(b)
			} else if name == "load" {
				m.push(&object.Builtin{Fn: func(args ...object.Object) object.Object {
					if len(args) != 1 {
						return newError("load expects 1 argument (path)")
					}
					p := args[0].Inspect()
					if s, ok := args[0].(*object.String); ok {
						p = s.Value
					}
					return LoadInto(p, env)
				}})
			} else if name == "use" {
				m.push(&object.Builtin{Fn: func(args ...object.Object) object.Object {
					if len(args) != 1 {
						return newError("use expects 1 argument (path)")
					}
					p := args[0].Inspect()
					if s, ok := args[0].(*object.String); ok {
						p = s.Value
					}
					return UseModule(p, env)
				}})
			} else {
				return newError("identifier not found: %s", name)
			}
		case opSet:
			idx := readU16(code, ip)
			ip += 2
			val := m.peek()
			env.Set(ch.names[idx], val)
		case opAdd, opSub, opMul, opDiv, opMod, opEq, opNeq, opLt, opLte, opGt, opGte, opIn:
			r := m.pop()
			l := m.pop()
			res := evalInfixExpression(opToInfix(op), l, r)
			if isError(res) {
				return res
			}
			m.push(res)
		case opMinus:
			r := m.pop()
			res := evalPrefixExpression("-", r)
			if isError(res) {
				return res
			}
			m.push(res)
		case opBang:
			r := m.pop()
			m.push(evalPrefixExpression("!", r))
		case opJump:
			ip = readU16(code, ip)
		case opJumpFalse:
			target := readU16(code, ip)
			ip += 2
			if !isTruthy(m.peek()) {
				ip = target
			}
		case opJumpTrue:
			target := readU16(code, ip)
			ip += 2
			if isTruthy(m.peek()) {
				ip = target
			}
		case opJumpOk:
			target := readU16(code, ip)
			ip += 2
			top := m.peek()
			if top != NULL && !isError(top) && !IsErrMap(top) {
				ip = target
			}
		case opToBool:
			m.push(nativeBoolToBooleanObject(isTruthy(m.pop())))
		case opArray:
			n := readU16(code, ip)
			ip += 2
			els := make([]object.Object, n)
			for i := n - 1; i >= 0; i-- {
				els[i] = m.pop()
			}
			m.push(&object.List{Elements: els})
		case opMap:
			n := readU16(code, ip)
			ip += 2
			pairs := make(map[string]object.Object)
			for i := 0; i < n; i++ {
				v := m.pop()
				k := m.pop()
				ks := k.Inspect()
				if s, ok := k.(*object.String); ok {
					ks = s.Value
				}
				pairs[ks] = v
			}
			m.push(&object.Map{Pairs: pairs})
		case opIndex:
			idx := m.pop()
			left := m.pop()
			res := evalIndexExpression(left, idx)
			if isError(res) {
				return res
			}
			m.push(res)
		case opSetIndex:
			idx := m.pop()
			obj := m.pop()
			val := m.peek()
			if mp, ok := obj.(*object.Map); ok {
				ks := idx.Inspect()
				if s, ok := idx.(*object.String); ok {
					ks = s.Value
				}
				mp.Pairs[ks] = val
			} else if list, ok := obj.(*object.List); ok {
				if i, ok := idx.(*object.Integer); ok && i.Value >= 0 && i.Value < int64(len(list.Elements)) {
					list.Elements[i.Value] = val
				}
			}
		case opProp:
			idx := readU16(code, ip)
			ip += 2
			left := m.pop()
			m.push(evalPropertyExpression(left, ch.names[idx]))
		case opSetProp:
			idx := readU16(code, ip)
			ip += 2
			obj := m.pop()
			val := m.peek()
			if mp, ok := obj.(*object.Map); ok {
				mp.Pairs[ch.names[idx]] = val
			}
		case opCall:
			n := readU16(code, ip)
			ip += 2
			args := make([]object.Object, n)
			for i := n - 1; i >= 0; i-- {
				args[i] = m.pop()
			}
			fn := m.pop()
			res := m.apply(fn, args, env)
			if isError(res) {
				return res
			}
			if rv, ok := res.(*object.ReturnValue); ok {
				res = rv.Value
			}
			m.push(res)
		case opReturn:
			return m.pop()
		case opClosure:
			idx := readU16(code, ip)
			ip += 2
			tmpl := ch.consts[idx].(*vmClosure)
			m.push(&vmClosure{ch: tmpl.ch, params: tmpl.params, env: env})
		case opDot:
			d := env.GetDot()
			if d == nil {
				return newError("context '.' is undefined outside pipeline or iteration")
			}
			m.push(d)
		case opDotField:
			idx := readU16(code, ip)
			ip += 2
			d := env.GetDot()
			if d == nil {
				return newError("context '.' is undefined outside pipeline or iteration")
			}
			m.push(evalDotField(d, ch.names[idx]))
		case opPipeFilter:
			pred := m.pop()
			left := m.pop()
			res := m.pipePred(left, pred, env, true)
			if isError(res) {
				return res
			}
			m.push(res)
		case opPipeMap:
			pred := m.pop()
			left := m.pop()
			res := m.pipePred(left, pred, env, false)
			if isError(res) {
				return res
			}
			m.push(res)
		case opPipeReduce:
			fn := m.pop()
			left := m.pop()
			list, ok := left.(*object.List)
			if !ok || len(list.Elements) == 0 {
				m.push(NULL)
				break
			}
			acc := list.Elements[0]
			for i := 1; i < len(list.Elements); i++ {
				acc = m.apply(fn, []object.Object{acc, list.Elements[i]}, env)
				if isError(acc) {
					return acc
				}
			}
			m.push(acc)
		case opPipeCall:
			n := readU16(code, ip)
			ip += 2
			extra := n - 1
			argsExtra := make([]object.Object, extra)
			for i := extra - 1; i >= 0; i-- {
				argsExtra[i] = m.pop()
			}
			fn := m.pop()
			left := m.pop()
			args := append([]object.Object{left}, argsExtra...)
			res := m.apply(fn, args, env)
			if isError(res) {
				return res
			}
			m.push(res)
		case opTool:
			nameIdx := readU16(code, ip)
			ip += 2
			n := readU16(code, ip)
			ip += 2
			args := make([]object.Object, n)
			for i := n - 1; i >= 0; i-- {
				args[i] = m.pop()
			}
			res := callToolByName(ch.names[nameIdx], args, env)
			if isError(res) {
				return res
			}
			m.push(res)
		case opUse:
			pathObj := m.pop()
			path := pathObj.Inspect()
			if s, ok := pathObj.(*object.String); ok {
				path = s.Value
			}
			res := UseModule(path, env)
			if isError(res) {
				return res
			}
			m.push(res)
		case opIter:
			subj := m.pop()
			items, err := collectIterable(subj)
			if err != nil {
				return err
			}
			m.push(&iterator{items: items})
		case opIterNext:
			target := readU16(code, ip)
			ip += 2
			it, ok := m.peek().(*iterator)
			if !ok || it.i >= len(it.items) {
				m.pop()
				ip = target
				break
			}
			item := it.items[it.i]
			it.i++
			m.push(item)
		case opSetDot:
			env.SetDot(m.peek())
		default:
			return newError("unknown opcode %d", op)
		}
	}
	if len(m.stack) == 0 {
		return NULL
	}
	return m.stack[len(m.stack)-1]
}

func (m *machine) push(v object.Object) { m.stack = append(m.stack, v) }
func (m *machine) pop() object.Object {
	v := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return v
}
func (m *machine) peek() object.Object { return m.stack[len(m.stack)-1] }

func (m *machine) apply(fn object.Object, args []object.Object, env *object.Environment) object.Object {
	switch f := fn.(type) {
	case *vmClosure:
		ex := object.NewEnclosedEnvironment(f.env)
		if env != nil {
			if d := env.GetDot(); d != nil {
				ex.SetDot(d)
			}
			if ex.File == "" {
				ex.File = env.File
				ex.Dir = env.Dir
			}
		}
		for i, p := range f.params {
			if i < len(args) {
				ex.Set(p, args[i])
			}
		}
		res := run(f.ch, ex)
		return unwrapReturnValue(res)
	default:
		return applyFunction(fn, args)
	}
}

func (m *machine) pipePred(left, pred object.Object, env *object.Environment, filter bool) object.Object {
	list, isList := left.(*object.List)
	runOne := func(item object.Object) object.Object {
		scoped := object.NewEnclosedEnvironment(env)
		scoped.SetDot(item)
		return m.apply(pred, nil, scoped)
	}
	if !isList {
		scoped := object.NewEnclosedEnvironment(env)
		scoped.SetDot(left)
		res := m.apply(pred, nil, scoped)
		if filter {
			if isTruthy(res) {
				return left
			}
			return NULL
		}
		return res
	}
	out := []object.Object{}
	for _, item := range list.Elements {
		res := runOne(item)
		if isError(res) {
			return res
		}
		if filter {
			if isTruthy(res) {
				out = append(out, item)
			}
		} else {
			out = append(out, res)
		}
	}
	return &object.List{Elements: out}
}

func evalDotField(dot object.Object, field string) object.Object {
	parts := strings.Split(field, ".")
	current := dot
	for _, part := range parts {
		m, ok := current.(*object.Map)
		if !ok {
			return NULL
		}
		val, exists := m.Pairs[part]
		if !exists {
			return NULL
		}
		current = val
	}
	return current
}

func opToInfix(op opcode) string {
	switch op {
	case opAdd:
		return "+"
	case opSub:
		return "-"
	case opMul:
		return "*"
	case opDiv:
		return "/"
	case opMod:
		return "%"
	case opEq:
		return "=="
	case opNeq:
		return "!="
	case opLt:
		return "<"
	case opLte:
		return "<="
	case opGt:
		return ">"
	case opGte:
		return ">="
	case opIn:
		return "in"
	}
	return "?"
}
