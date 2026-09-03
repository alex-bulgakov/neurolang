package evaluator

import (
	"fmt"
	"neurolang/pkg/ast"
	"neurolang/pkg/lexer"
	"neurolang/pkg/object"
	"neurolang/pkg/parser"
	"neurolang/pkg/tools"
	"os"
	"strings"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.WhileStatement:
		return evalWhileStatement(node, env)

	case *ast.BreakStatement:
		return &object.BreakSignal{}

	case *ast.ContinueStatement:
		return &object.ContinueSignal{}

	case *ast.AssignStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
		return val

	// Literals
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}

	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.BooleanLiteral:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.NullLiteral:
		return NULL

	case *ast.ListLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.List{Elements: elements}

	case *ast.MapLiteral:
		pairs := make(map[string]object.Object)
		for keyNode, valNode := range node.Pairs {
			keyObj := Eval(keyNode, env)
			if isError(keyObj) {
				return keyObj
			}
			keyStr := keyObj.Inspect()
			if str, ok := keyObj.(*object.String); ok {
				keyStr = str.Value
			}

			valObj := Eval(valNode, env)
			if isError(valObj) {
				return valObj
			}
			pairs[keyStr] = valObj
		}
		return &object.Map{Pairs: pairs}

	// Identifiers
	case *ast.Identifier:
		return evalIdentifier(node, env)

	// Prefix and Infix
	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)

	// Dataflow Pipe: left | right
	case *ast.PipeExpression:
		return evalPipeExpression(node, env)

	// Context dot: . or .field or .user.name
	case *ast.DotExpression:
		return evalDotExpression(node, env)

	// Function and Call
	case *ast.ArrowFunctionLiteral:
		return &object.Function{
			Parameters: node.Parameters,
			Body:       node.Body,
			Env:        env,
		}

	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args)

	// Tool call: !tool.name(...)
	case *ast.ToolCallExpression:
		return evalToolCall(node, env)

	// Index: arr[i], map[k]
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)

	// Property: obj.prop
	case *ast.PropertyExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		return evalPropertyExpression(left, node.Property)

	// Control flow
	case *ast.IfExpression:
		return evalIfExpression(node, env)

	case *ast.MatchExpression:
		return evalMatchExpression(node, env)

	// Naked Combinators outside pipe (evaluated against current dot)
	case *ast.FilterExpression:
		dot := env.GetDot()
		if dot == nil {
			return newError("cannot evaluate filter outside of a pipeline or context")
		}
		return evalFilter(dot, node.Predicate, env)

	case *ast.MapExpression:
		dot := env.GetDot()
		if dot == nil {
			return newError("cannot evaluate map outside of a pipeline or context")
		}
		return evalMap(dot, node.Transform, env)

	case *ast.ReduceExpression:
		dot := env.GetDot()
		if dot == nil {
			return newError("cannot evaluate reduce outside of a pipeline or context")
		}
		return evalReduce(dot, node.Reducer, env)
	}

	return nil
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ || rt == object.BREAK_SIGNAL_OBJ || rt == object.CONTINUE_SIGNAL_OBJ {
				return result
			}
		}
	}

	return result
}

func evalWhileStatement(ws *ast.WhileStatement, env *object.Environment) object.Object {
	for {
		condition := Eval(ws.Condition, env)
		if isError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break
		}

		res := Eval(ws.Body, env)
		if res != nil {
			if res.Type() == object.BREAK_SIGNAL_OBJ {
				break
			}
			if res.Type() == object.RETURN_VALUE_OBJ || res.Type() == object.ERROR_OBJ {
				return res
			}
			// CONTINUE_SIGNAL moves to next iteration
		}
	}

	return NULL
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	if node.Value == "load" {
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("load expects 1 argument (path)")
				}
				path := args[0].Inspect()
				if s, ok := args[0].(*object.String); ok {
					path = s.Value
				}
				bytes, err := os.ReadFile(path)
				if err != nil {
					return newError("load error: %s", err.Error())
				}
				l := lexer.New(string(bytes))
				p := parser.New(l)
				prog := p.ParseProgram()
				if len(p.Errors()) > 0 {
					return newError("load parse error: %s", p.Errors()[0])
				}
				return Eval(prog, env)
			},
		}
	}

	return newError("identifier not found: %s", node.Value)
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		if b, ok := right.(*object.Boolean); ok {
			return nativeBoolToBooleanObject(!b.Value)
		}
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch obj := right.(type) {
	case *object.Integer:
		return &object.Integer{Value: -obj.Value}
	case *object.Float:
		return &object.Float{Value: -obj.Value}
	default:
		return newError("unknown operator: -%s", right.Type())
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	// Logical AND / OR
	if operator == "&&" {
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	}
	if operator == "||" {
		return nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))
	}

	// Equality check for any types
	if operator == "==" {
		return nativeBoolToBooleanObject(areEqual(left, right))
	}
	if operator == "!=" {
		return nativeBoolToBooleanObject(!areEqual(left, right))
	}

	// Number operations (Int and Float)
	if isNumeric(left) && isNumeric(right) {
		return evalNumericInfixExpression(operator, left, right)
	}

	// String concatenation
	if left.Type() == object.STRING_OBJ && operator == "+" {
		lVal := left.(*object.String).Value
		rVal := right.Inspect()
		if rStr, ok := right.(*object.String); ok {
			rVal = rStr.Value
		}
		return &object.String{Value: lVal + rVal}
	}

	// List concatenation
	if left.Type() == object.LIST_OBJ && operator == "+" {
		leftList := left.(*object.List)
		if rightList, ok := right.(*object.List); ok {
			combined := make([]object.Object, 0, len(leftList.Elements)+len(rightList.Elements))
			combined = append(combined, leftList.Elements...)
			combined = append(combined, rightList.Elements...)
			return &object.List{Elements: combined}
		}
		combined := make([]object.Object, 0, len(leftList.Elements)+1)
		combined = append(combined, leftList.Elements...)
		combined = append(combined, right)
		return &object.List{Elements: combined}
	}

	return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
}

func evalNumericInfixExpression(operator string, left, right object.Object) object.Object {
	isLeftFloat := left.Type() == object.FLOAT_OBJ
	isRightFloat := right.Type() == object.FLOAT_OBJ

	if isLeftFloat || isRightFloat {
		var lVal, rVal float64
		if isLeftFloat {
			lVal = left.(*object.Float).Value
		} else {
			lVal = float64(left.(*object.Integer).Value)
		}
		if isRightFloat {
			rVal = right.(*object.Float).Value
		} else {
			rVal = float64(right.(*object.Integer).Value)
		}

		switch operator {
		case "+":
			return &object.Float{Value: lVal + rVal}
		case "-":
			return &object.Float{Value: lVal - rVal}
		case "*":
			return &object.Float{Value: lVal * rVal}
		case "/":
			if rVal == 0 {
				return newError("division by zero")
			}
			return &object.Float{Value: lVal / rVal}
		case "<":
			return nativeBoolToBooleanObject(lVal < rVal)
		case "<=":
			return nativeBoolToBooleanObject(lVal <= rVal)
		case ">":
			return nativeBoolToBooleanObject(lVal > rVal)
		case ">=":
			return nativeBoolToBooleanObject(lVal >= rVal)
		default:
			return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
		}
	}

	// Both are integers
	lVal := left.(*object.Integer).Value
	rVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: lVal + rVal}
	case "-":
		return &object.Integer{Value: lVal - rVal}
	case "*":
		return &object.Integer{Value: lVal * rVal}
	case "/":
		if rVal == 0 {
			return newError("division by zero")
		}
		return &object.Integer{Value: lVal / rVal}
	case "%":
		if rVal == 0 {
			return newError("modulo by zero")
		}
		return &object.Integer{Value: lVal % rVal}
	case "<":
		return nativeBoolToBooleanObject(lVal < rVal)
	case "<=":
		return nativeBoolToBooleanObject(lVal <= rVal)
	case ">":
		return nativeBoolToBooleanObject(lVal > rVal)
	case ">=":
		return nativeBoolToBooleanObject(lVal >= rVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

// Dataflow Pipeline: left | right
func evalPipeExpression(node *ast.PipeExpression, env *object.Environment) object.Object {
	leftVal := Eval(node.Left, env)
	if isError(leftVal) {
		return leftVal
	}

	switch rightNode := node.Right.(type) {
	case *ast.FilterExpression:
		return evalFilter(leftVal, rightNode.Predicate, env)

	case *ast.MapExpression:
		return evalMap(leftVal, rightNode.Transform, env)

	case *ast.ReduceExpression:
		return evalReduce(leftVal, rightNode.Reducer, env)

	case *ast.ToolCallExpression:
		// Pipeline into tool: pass leftVal as first argument
		args := []object.Object{leftVal}
		for _, argNode := range rightNode.Arguments {
			argVal := Eval(argNode, env)
			if isError(argVal) {
				return argVal
			}
			args = append(args, argVal)
		}
		return callToolByName(rightNode.ToolName, args, env)

	case *ast.CallExpression:
		// Pipeline into function call: e.g. x | f(y) -> f(x, y)
		fnObj := Eval(rightNode.Function, env)
		if isError(fnObj) {
			return fnObj
		}
		args := []object.Object{leftVal}
		for _, argNode := range rightNode.Arguments {
			argVal := Eval(argNode, env)
			if isError(argVal) {
				return argVal
			}
			args = append(args, argVal)
		}
		return applyFunction(fnObj, args)

	default:
		// If right is an identifier or function expression: x | f -> f(x)
		rightVal := Eval(node.Right, env)
		if isError(rightVal) {
			return rightVal
		}
		if fn, ok := rightVal.(*object.Function); ok {
			return applyFunction(fn, []object.Object{leftVal})
		}
		if builtin, ok := rightVal.(*object.Builtin); ok {
			return builtin.Fn(leftVal)
		}
		return newError("right side of pipe '|' must be a function, tool, map (@), filter (?), or reduce (&)")
	}
}

func evalFilter(subject object.Object, pred ast.Expression, env *object.Environment) object.Object {
	list, isList := subject.(*object.List)
	if !isList {
		// Filter single item
		scopedEnv := object.NewEnclosedEnvironment(env)
		scopedEnv.SetDot(subject)
		res := Eval(pred, scopedEnv)
		if isError(res) {
			return res
		}
		if isTruthy(res) {
			return subject
		}
		return NULL
	}

	filtered := []object.Object{}
	for _, item := range list.Elements {
		scopedEnv := object.NewEnclosedEnvironment(env)
		scopedEnv.SetDot(item)
		res := Eval(pred, scopedEnv)
		if isError(res) {
			return res
		}
		if isTruthy(res) {
			filtered = append(filtered, item)
		}
	}
	return &object.List{Elements: filtered}
}

func evalMap(subject object.Object, transform ast.Expression, env *object.Environment) object.Object {
	list, isList := subject.(*object.List)
	if !isList {
		// Map single item
		scopedEnv := object.NewEnclosedEnvironment(env)
		scopedEnv.SetDot(subject)
		return Eval(transform, scopedEnv)
	}

	mapped := make([]object.Object, len(list.Elements))
	for i, item := range list.Elements {
		scopedEnv := object.NewEnclosedEnvironment(env)
		scopedEnv.SetDot(item)
		res := Eval(transform, scopedEnv)
		if isError(res) {
			return res
		}
		mapped[i] = res
	}
	return &object.List{Elements: mapped}
}

func evalReduce(subject object.Object, reducerNode ast.Expression, env *object.Environment) object.Object {
	list, ok := subject.(*object.List)
	if !ok {
		return newError("cannot reduce non-list object %s", subject.Type())
	}
	if len(list.Elements) == 0 {
		return NULL
	}

	reducerObj := Eval(reducerNode, env)
	if isError(reducerObj) {
		return reducerObj
	}

	acc := list.Elements[0]
	for i := 1; i < len(list.Elements); i++ {
		item := list.Elements[i]
		res := applyFunction(reducerObj, []object.Object{acc, item})
		if isError(res) {
			return res
		}
		acc = res
	}
	return acc
}

func evalDotExpression(node *ast.DotExpression, env *object.Environment) object.Object {
	dot := env.GetDot()
	if dot == nil {
		return newError("context '.' is undefined outside pipeline or iteration")
	}

	if node.Field == "" {
		return dot
	}

	// Resolve chained field: e.g. "user.address.city"
	parts := strings.Split(node.Field, ".")
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

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.LIST_OBJ && index.Type() == object.INTEGER_OBJ:
		list := left.(*object.List)
		idx := index.(*object.Integer).Value
		max := int64(len(list.Elements) - 1)
		if idx < 0 || idx > max {
			return NULL
		}
		return list.Elements[idx]

	case left.Type() == object.MAP_OBJ && index.Type() == object.STRING_OBJ:
		m := left.(*object.Map)
		key := index.(*object.String).Value
		val, ok := m.Pairs[key]
		if !ok {
			return NULL
		}
		return val

	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		s := left.(*object.String).Value
		idx := index.(*object.Integer).Value
		if idx < 0 || idx >= int64(len(s)) {
			return NULL
		}
		return &object.String{Value: string(s[idx])}

	default:
		return newError("index operator not supported: %s[%s]", left.Type(), index.Type())
	}
}

func evalPropertyExpression(left object.Object, property string) object.Object {
	m, ok := left.(*object.Map)
	if !ok {
		return NULL
	}
	val, exists := m.Pairs[property]
	if !exists {
		return NULL
	}
	return val
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	}
	return NULL
}

func evalMatchExpression(me *ast.MatchExpression, env *object.Environment) object.Object {
	subject := Eval(me.Subject, env)
	if isError(subject) {
		return subject
	}

	for _, c := range me.Cases {
		// Wildcard check: '_'
		if ident, ok := c.Pattern.(*ast.Identifier); ok && ident.Value == "_" {
			return Eval(c.Body, env)
		}

		patternVal := Eval(c.Pattern, env)
		if isError(patternVal) {
			return patternVal
		}

		if areEqual(subject, patternVal) {
			return Eval(c.Body, env)
		}
	}

	return NULL
}

func evalToolCall(node *ast.ToolCallExpression, env *object.Environment) object.Object {
	args := evalExpressions(node.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}
	return callToolByName(node.ToolName, args, env)
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch function := fn.(type) {
	case *object.Function:
		extendedEnv := extendFunctionEnv(function, args)
		evaluated := Eval(function.Body, extendedEnv)
		return unwrapReturnValue(evaluated)

	case *object.Builtin:
		return function.Fn(args...)

	default:
		return newError("not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)
	for i, param := range fn.Parameters {
		if i < len(args) {
			env.Set(param.Value, args[i])
		}
	}
	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		if b, ok := obj.(*object.Boolean); ok {
			return b.Value
		}
		if i, ok := obj.(*object.Integer); ok {
			return i.Value != 0
		}
		if f, ok := obj.(*object.Float); ok {
			return f.Value != 0.0
		}
		if s, ok := obj.(*object.String); ok {
			return len(s.Value) > 0
		}
		if l, ok := obj.(*object.List); ok {
			return len(l.Elements) > 0
		}
		return true
	}
}

func areEqual(a, b object.Object) bool {
	if a == b {
		return true
	}
	if a.Type() != b.Type() {
		// Check integer and float comparison
		if isNumeric(a) && isNumeric(b) {
			return getFloatVal(a) == getFloatVal(b)
		}
		return false
	}
	switch a := a.(type) {
	case *object.Integer:
		return a.Value == b.(*object.Integer).Value
	case *object.Float:
		return a.Value == b.(*object.Float).Value
	case *object.Boolean:
		return a.Value == b.(*object.Boolean).Value
	case *object.String:
		return a.Value == b.(*object.String).Value
	case *object.Null:
		return true
	}
	return false
}

func isNumeric(obj object.Object) bool {
	return obj.Type() == object.INTEGER_OBJ || obj.Type() == object.FLOAT_OBJ
}

func getFloatVal(obj object.Object) float64 {
	if i, ok := obj.(*object.Integer); ok {
		return float64(i.Value)
	}
	if f, ok := obj.(*object.Float); ok {
		return f.Value
	}
	return 0
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func newError(format string, a ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func callToolByName(name string, args []object.Object, env *object.Environment) object.Object {
	return tools.Call(name, args, env)
}
