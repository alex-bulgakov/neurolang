package evaluator

import (
	"encoding/json"
	"fmt"
	"neurolang/pkg/object"
	"sort"
	"strings"
)

var builtins = map[string]*object.Builtin{
	"len": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.String:
				return &object.Integer{Value: int64(len(arg.Value))}
			case *object.List:
				return &object.Integer{Value: int64(len(arg.Elements))}
			case *object.Map:
				return &object.Integer{Value: int64(len(arg.Pairs))}
			default:
				return newError("argument to `len` not supported, got %s", args[0].Type())
			}
		},
	},

	"print": {
		Fn: func(args ...object.Object) object.Object {
			for i, arg := range args {
				if i > 0 {
					fmt.Print(" ")
				}
				if str, ok := arg.(*object.String); ok {
					fmt.Print(str.Value)
				} else {
					fmt.Print(arg.Inspect())
				}
			}
			fmt.Println()
			if len(args) == 1 {
				return args[0]
			}
			return &object.Null{}
		},
	},

	"type": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: string(args[0].Type())}
		},
	},

	"json": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			data, err := json.MarshalIndent(args[0].ToInterface(), "", "  ")
			if err != nil {
				return newError("json serialization failed: %s", err.Error())
			}
			return &object.String{Value: string(data)}
		},
	},

	"parse_json": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return newError("argument to `parse_json` must be STRING, got %s", args[0].Type())
			}
			var parsed any
			err := json.Unmarshal([]byte(str.Value), &parsed)
			if err != nil {
				return newError("invalid json: %s", err.Error())
			}
			return object.FromGoValue(parsed)
		},
	},

	"range": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("wrong number of arguments to range. got=%d, want=1 or 2", len(args))
			}
			var start, end int64
			if len(args) == 1 {
				start = 0
				e, ok := args[0].(*object.Integer)
				if !ok {
					return newError("argument to range must be INTEGER, got %s", args[0].Type())
				}
				end = e.Value
			} else {
				s, ok1 := args[0].(*object.Integer)
				e, ok2 := args[1].(*object.Integer)
				if !ok1 || !ok2 {
					return newError("arguments to range must be INTEGER")
				}
				start = s.Value
				end = e.Value
			}

			elements := []object.Object{}
			for i := start; i < end; i++ {
				elements = append(elements, &object.Integer{Value: i})
			}
			return &object.List{Elements: elements}
		},
	},

	"keys": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			m, ok := args[0].(*object.Map)
			if !ok {
				return newError("argument to keys must be MAP, got %s", args[0].Type())
			}
			keys := make([]string, 0, len(m.Pairs))
			for k := range m.Pairs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			res := make([]object.Object, len(keys))
			for i, k := range keys {
				res[i] = &object.String{Value: k}
			}
			return &object.List{Elements: res}
		},
	},

	"values": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			m, ok := args[0].(*object.Map)
			if !ok {
				return newError("argument to values must be MAP, got %s", args[0].Type())
			}
			res := make([]object.Object, 0, len(m.Pairs))
			for _, v := range m.Pairs {
				res = append(res, v)
			}
			return &object.List{Elements: res}
		},
	},

	"join": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			list, ok1 := args[0].(*object.List)
			sep, ok2 := args[1].(*object.String)
			if !ok1 || !ok2 {
				return newError("join expects (LIST, STRING)")
			}
			parts := make([]string, len(list.Elements))
			for i, el := range list.Elements {
				parts[i] = el.Inspect()
			}
			return &object.String{Value: strings.Join(parts, sep.Value)}
		},
	},

	"split": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			str, ok1 := args[0].(*object.String)
			sep, ok2 := args[1].(*object.String)
			if !ok1 || !ok2 {
				return newError("split expects (STRING, STRING)")
			}
			parts := strings.Split(str.Value, sep.Value)
			elements := make([]object.Object, len(parts))
			for i, p := range parts {
				elements[i] = &object.String{Value: p}
			}
			return &object.List{Elements: elements}
		},
	},
}
