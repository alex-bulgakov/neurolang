package object

import (
	"bytes"
	"encoding/json"
	"fmt"
	"neurolang/pkg/ast"
	"sort"
	"strings"
)

type ObjectType string

const (
	INTEGER_OBJ      = "INTEGER"
	FLOAT_OBJ        = "FLOAT"
	BOOLEAN_OBJ      = "BOOLEAN"
	STRING_OBJ       = "STRING"
	NULL_OBJ         = "NULL"
	RETURN_VALUE_OBJ = "RETURN_VALUE"
	ERROR_OBJ        = "ERROR"
	FUNCTION_OBJ     = "FUNCTION"
	BUILTIN_OBJ      = "BUILTIN"
	LIST_OBJ         = "LIST"
	MAP_OBJ          = "MAP"
)

type Object interface {
	Type() ObjectType
	Inspect() string
	ToInterface() any
}

// Integer
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType   { return INTEGER_OBJ }
func (i *Integer) Inspect() string    { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) ToInterface() any   { return i.Value }

// Float
type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType   { return FLOAT_OBJ }
func (f *Float) Inspect() string    { return fmt.Sprintf("%g", f.Value) }
func (f *Float) ToInterface() any   { return f.Value }

// Boolean
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType   { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string    { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) ToInterface() any   { return b.Value }

// Null
type Null struct{}

func (n *Null) Type() ObjectType   { return NULL_OBJ }
func (n *Null) Inspect() string    { return "null" }
func (n *Null) ToInterface() any   { return nil }

// String
type String struct {
	Value string
}

func (s *String) Type() ObjectType   { return STRING_OBJ }
func (s *String) Inspect() string    { return fmt.Sprintf("%q", s.Value) }
func (s *String) ToInterface() any   { return s.Value }

// ReturnValue
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType   { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string    { return rv.Value.Inspect() }
func (rv *ReturnValue) ToInterface() any   { return rv.Value.ToInterface() }

// Error
type Error struct {
	Message string
}

func (e *Error) Type() ObjectType   { return ERROR_OBJ }
func (e *Error) Inspect() string    { return "Error: " + e.Message }
func (e *Error) ToInterface() any   { return e.Message }

// Function
type Function struct {
	Parameters []*ast.Identifier
	Body       ast.Expression
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") -> ")
	out.WriteString(f.Body.String())
	return out.String()
}
func (f *Function) ToInterface() any { return f.Inspect() }

// BuiltinFunction
type BuiltinFunction func(args ...Object) Object

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType   { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string    { return "builtin function" }
func (b *Builtin) ToInterface() any   { return "builtin" }

// List
type List struct {
	Elements []Object
}

func (l *List) Type() ObjectType { return LIST_OBJ }
func (l *List) Inspect() string {
	var out bytes.Buffer
	elements := []string{}
	for _, e := range l.Elements {
		elements = append(elements, e.Inspect())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")
	return out.String()
}
func (l *List) ToInterface() any {
	res := make([]any, len(l.Elements))
	for i, e := range l.Elements {
		res[i] = e.ToInterface()
	}
	return res
}

// Map
type Map struct {
	Pairs map[string]Object
}

func (m *Map) Type() ObjectType { return MAP_OBJ }
func (m *Map) Inspect() string {
	var out bytes.Buffer
	keys := make([]string, 0, len(m.Pairs))
	for k := range m.Pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := []string{}
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s: %s", k, m.Pairs[k].Inspect()))
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}
func (m *Map) ToInterface() any {
	res := make(map[string]any)
	for k, v := range m.Pairs {
		res[k] = v.ToInterface()
	}
	return res
}

// Helpers for Go/JSON interop
func FromGoValue(v any) Object {
	if v == nil {
		return &Null{}
	}
	switch val := v.(type) {
	case bool:
		return &Boolean{Value: val}
	case int:
		return &Integer{Value: int64(val)}
	case int32:
		return &Integer{Value: int64(val)}
	case int64:
		return &Integer{Value: val}
	case float32:
		return &Float{Value: float64(val)}
	case float64:
		// If float is whole number, can keep as float or int; keep float
		return &Float{Value: val}
	case string:
		return &String{Value: val}
	case []any:
		elements := make([]Object, len(val))
		for i, el := range val {
			elements[i] = FromGoValue(el)
		}
		return &List{Elements: elements}
	case map[string]any:
		pairs := make(map[string]Object)
		for k, el := range val {
			pairs[k] = FromGoValue(el)
		}
		return &Map{Pairs: pairs}
	default:
		// Fallback via JSON marshaling
		data, err := json.Marshal(v)
		if err != nil {
			return &String{Value: fmt.Sprintf("%v", v)}
		}
		var parsed any
		if err := json.Unmarshal(data, &parsed); err == nil {
			return FromGoValue(parsed)
		}
		return &String{Value: fmt.Sprintf("%v", v)}
	}
}
