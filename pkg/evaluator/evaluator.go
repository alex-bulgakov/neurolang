package evaluator

import (
	"fmt"
	"neurolang/pkg/object"
	"neurolang/pkg/tools"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	NULL      = &object.Null{}
	TRUE      = &object.Boolean{Value: true}
	FALSE     = &object.Boolean{Value: false}
	activeEnv *object.Environment
)

func collectIterable(iterable object.Object) ([]object.Object, *object.Error) {
	switch it := iterable.(type) {
	case *object.List:
		return it.Elements, nil
	case *object.String:
		runes := []rune(it.Value)
		items := make([]object.Object, len(runes))
		for i, r := range runes {
			items[i] = &object.String{Value: string(r)}
		}
		return items, nil
	case *object.Map:
		keys := make([]string, 0, len(it.Pairs))
		for k := range it.Pairs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		items := make([]object.Object, len(keys))
		for i, k := range keys {
			items[i] = &object.String{Value: k}
		}
		return items, nil
	default:
		return nil, newError("cannot iterate over %s", iterable.Type())
	}
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
	if operator == "in" {
		return evalInOperator(left, right)
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

func evalInOperator(left, right object.Object) object.Object {
	switch r := right.(type) {
	case *object.List:
		for _, el := range r.Elements {
			if areEqual(left, el) {
				return TRUE
			}
		}
		return FALSE
	case *object.Map:
		key := left.Inspect()
		if s, ok := left.(*object.String); ok {
			key = s.Value
		}
		_, exists := r.Pairs[key]
		return nativeBoolToBooleanObject(exists)
	case *object.String:
		l, ok := left.(*object.String)
		if !ok {
			return newError("in: left operand must be STRING when searching a STRING")
		}
		return nativeBoolToBooleanObject(strings.Contains(r.Value, l.Value))
	default:
		return newError("in: cannot search in %s", right.Type())
	}
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
	if e, ok := left.(*object.Error); ok {
		left = e.AsMap()
	}
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

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch function := fn.(type) {
	case *vmClosure:
		ex := object.NewEnclosedEnvironment(function.env)
		if activeEnv != nil {
			if d := activeEnv.GetDot(); d != nil {
				ex.SetDot(d)
			}
			if ex.File == "" {
				ex.File = activeEnv.File
				ex.Dir = activeEnv.Dir
			}
		}
		for i, p := range function.params {
			if i < len(args) {
				ex.Set(p, args[i])
			}
		}
		return unwrapReturnValue(run(function.ch, ex))

	case *object.Builtin:
		return function.Fn(args...)

	default:
		return newError("not a function: %s", fn.Type())
	}
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
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
	return areEqualDepth(a, b, 0)
}

func areEqualDepth(a, b object.Object, depth int) bool {
	if a == b {
		return true
	}
	if depth > 64 {
		return false
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type() != b.Type() {
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
	case *object.List:
		bList := b.(*object.List)
		if len(a.Elements) != len(bList.Elements) {
			return false
		}
		for i := range a.Elements {
			if !areEqualDepth(a.Elements[i], bList.Elements[i], depth+1) {
				return false
			}
		}
		return true
	case *object.Map:
		bMap := b.(*object.Map)
		if len(a.Pairs) != len(bMap.Pairs) {
			return false
		}
		for k, vA := range a.Pairs {
			vB, ok := bMap.Pairs[k]
			if !ok || !areEqualDepth(vA, vB, depth+1) {
				return false
			}
		}
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

func ResolveModulePath(path string, env *object.Environment) (string, error) {
	variants := []string{path}
	if !strings.HasSuffix(path, ".nl") {
		variants = append(variants, path+".nl")
	}

	var candidates []string
	add := func(p string) {
		if p != "" {
			candidates = append(candidates, p)
		}
	}
	for _, v := range variants {
		add(v)
		if env != nil && env.Dir != "" {
			add(filepath.Join(env.Dir, v))
		}
		if cwd, err := os.Getwd(); err == nil {
			add(filepath.Join(cwd, v))
			dir := cwd
			for {
				add(filepath.Join(dir, v))
				if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
					break
				}
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				dir = parent
			}
		}
		if exe, err := os.Executable(); err == nil {
			add(filepath.Join(filepath.Dir(exe), v))
		}
	}

	seen := map[string]bool{}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if st, err := os.Stat(abs); err == nil && !st.IsDir() {
			return abs, nil
		}
	}
	return "", fmt.Errorf("module not found: %s", path)
}

func UseModule(path string, from *object.Environment) object.Object {
	resolved, err := ResolveModulePath(path, from)
	if err != nil {
		return newError("use: %s", err.Error())
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return newError("use: %s", err.Error())
	}
	return useViaStd(string(data), resolved)
}

func LoadInto(path string, env *object.Environment) object.Object {
	resolved, err := ResolveModulePath(path, env)
	if err != nil {
		return newError("load: %s", err.Error())
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return newError("load error: %s", err.Error())
	}
	return loadViaStd(string(data), resolved, env)
}

func ErrMap(msg string, line, col int) *object.Map {
	pairs := map[string]object.Object{
		"err": &object.String{Value: msg},
	}
	if line > 0 {
		pairs["line"] = &object.Integer{Value: int64(line)}
		pairs["col"] = &object.Integer{Value: int64(col)}
	}
	return &object.Map{Pairs: pairs}
}

func IsErrMap(obj object.Object) bool {
	if obj == nil {
		return false
	}
	if obj.Type() == object.ERROR_OBJ {
		return true
	}
	m, ok := obj.(*object.Map)
	if !ok {
		return false
	}
	v, exists := m.Pairs["err"]
	if !exists || v == nil || v == NULL {
		return false
	}
	if s, ok := v.(*object.String); ok {
		return s.Value != ""
	}
	return true
}

func FormatErr(obj object.Object) string {
	if e, ok := obj.(*object.Error); ok {
		return e.Inspect()
	}
	m, ok := obj.(*object.Map)
	if !ok {
		return obj.Inspect()
	}
	msg := ""
	if s, ok := m.Pairs["err"].(*object.String); ok {
		msg = s.Value
	} else if v, ok := m.Pairs["err"]; ok && v != nil {
		msg = v.Inspect()
	}
	line, col := int64(0), int64(0)
	if i, ok := m.Pairs["line"].(*object.Integer); ok {
		line = i.Value
	}
	if i, ok := m.Pairs["col"].(*object.Integer); ok {
		col = i.Value
	}
	if line > 0 {
		return fmt.Sprintf("Error: line %d, col %d: %s", line, col, msg)
	}
	return "Error: " + msg
}
