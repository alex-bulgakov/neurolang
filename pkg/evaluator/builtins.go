package evaluator

import (
	"encoding/json"
	"fmt"
	"neurolang/pkg/object"
	"neurolang/pkg/tools"
	"sort"
	"strconv"
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
				return &object.Integer{Value: int64(len([]rune(arg.Value)))}
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
				if s, ok := el.(*object.String); ok {
					parts[i] = s.Value
				} else {
					parts[i] = el.Inspect()
				}
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

	"slice": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return newError("slice expects (seq, start, [end])")
			}
			startObj, ok1 := args[1].(*object.Integer)
			if !ok1 {
				return newError("slice start must be INTEGER")
			}
			start := int(startObj.Value)

			switch seq := args[0].(type) {
			case *object.String:
				runes := []rune(seq.Value)
				length := len(runes)
				end := length
				if len(args) == 3 {
					endObj, ok2 := args[2].(*object.Integer)
					if !ok2 {
						return newError("slice end must be INTEGER")
					}
					end = int(endObj.Value)
				}
				if start < 0 {
					start = 0
				}
				if start > length {
					start = length
				}
				if end < start {
					end = start
				}
				if end > length {
					end = length
				}
				return &object.String{Value: string(runes[start:end])}

			case *object.List:
				length := len(seq.Elements)
				end := length
				if len(args) == 3 {
					endObj, ok2 := args[2].(*object.Integer)
					if !ok2 {
						return newError("slice end must be INTEGER")
					}
					end = int(endObj.Value)
				}
				if start < 0 {
					start = 0
				}
				if start > length {
					start = length
				}
				if end < start {
					end = start
				}
				if end > length {
					end = length
				}
				res := make([]object.Object, end-start)
				copy(res, seq.Elements[start:end])
				return &object.List{Elements: res}

			default:
				return newError("slice expects STRING or LIST, got %s", args[0].Type())
			}
		},
	},

	"append": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("append expects (LIST, elem)")
			}
			list, ok := args[0].(*object.List)
			if !ok {
				return newError("first argument to append must be LIST, got %s", args[0].Type())
			}
			res := make([]object.Object, 0, len(list.Elements)+1)
			res = append(res, list.Elements...)
			res = append(res, args[1])
			return &object.List{Elements: res}
		},
	},

	"ord": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("ord expects 1 argument")
			}
			str, ok := args[0].(*object.String)
			if !ok || len(str.Value) == 0 {
				return newError("ord expects non-empty STRING")
			}
			runes := []rune(str.Value)
			return &object.Integer{Value: int64(runes[0])}
		},
	},

	"chr": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("chr expects 1 argument")
			}
			code, ok := args[0].(*object.Integer)
			if !ok {
				return newError("chr expects INTEGER")
			}
			return &object.String{Value: string(rune(code.Value))}
		},
	},

	"is_digit": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("is_digit expects 1 argument")
			}
			str, ok := args[0].(*object.String)
			if !ok || len(str.Value) == 0 {
				return &object.Boolean{Value: false}
			}
			r := []rune(str.Value)[0]
			return &object.Boolean{Value: '0' <= r && r <= '9'}
		},
	},

	"is_alpha": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("is_alpha expects 1 argument")
			}
			str, ok := args[0].(*object.String)
			if !ok || len(str.Value) == 0 {
				return &object.Boolean{Value: false}
			}
			r := []rune(str.Value)[0]
			isA := ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || r == '_' || r == '$'
			return &object.Boolean{Value: isA}
		},
	},

	"is_space": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("is_space expects 1 argument")
			}
			str, ok := args[0].(*object.String)
			if !ok || len(str.Value) == 0 {
				return &object.Boolean{Value: false}
			}
			r := []rune(str.Value)[0]
			return &object.Boolean{Value: r == ' ' || r == '\t' || r == '\n' || r == '\r'}
		},
	},
}

func init() {
	builtins["int"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("int expects 1 argument")
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(arg.Value)}
			case *object.Boolean:
				if arg.Value {
					return &object.Integer{Value: 1}
				}
				return &object.Integer{Value: 0}
			case *object.String:
				n, err := strconv.ParseInt(strings.TrimSpace(arg.Value), 10, 64)
				if err != nil {
					return newError("int: cannot parse %q", arg.Value)
				}
				return &object.Integer{Value: n}
			default:
				return newError("int: cannot convert %s", args[0].Type())
			}
		},
	}

	builtins["float"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("float expects 1 argument")
			}
			switch arg := args[0].(type) {
			case *object.Float:
				return arg
			case *object.Integer:
				return &object.Float{Value: float64(arg.Value)}
			case *object.String:
				n, err := strconv.ParseFloat(strings.TrimSpace(arg.Value), 64)
				if err != nil {
					return newError("float: cannot parse %q", arg.Value)
				}
				return &object.Float{Value: n}
			default:
				return newError("float: cannot convert %s", args[0].Type())
			}
		},
	}

	builtins["str"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("str expects 1 argument")
			}
			if s, ok := args[0].(*object.String); ok {
				return s
			}
			return &object.String{Value: args[0].Inspect()}
		},
	}

	builtins["copy"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("copy expects 1 argument")
			}
			switch arg := args[0].(type) {
			case *object.Map:
				pairs := make(map[string]object.Object, len(arg.Pairs))
				for k, v := range arg.Pairs {
					pairs[k] = v
				}
				return &object.Map{Pairs: pairs}
			case *object.List:
				els := make([]object.Object, len(arg.Elements))
				copy(els, arg.Elements)
				return &object.List{Elements: els}
			default:
				return arg
			}
		},
	}

	builtins["apply"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("apply expects (fn, args_list)")
			}
			list, ok := args[1].(*object.List)
			if !ok {
				return newError("apply: second argument must be LIST")
			}
			return applyFunction(args[0], list.Elements)
		},
	}

	builtins["tool_call"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("tool_call expects (name, args_list)")
			}
			name := args[0].Inspect()
			if s, ok := args[0].(*object.String); ok {
				name = s.Value
			}
			list, ok := args[1].(*object.List)
			if !ok {
				return newError("tool_call: second argument must be LIST")
			}
			return tools.Call(name, list.Elements, nil)
		},
	}

	builtins["use"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("use expects 1 argument (path)")
			}
			path := args[0].Inspect()
			if s, ok := args[0].(*object.String); ok {
				path = s.Value
			}
			return UseModule(path, activeEnv)
		},
	}

	builtins["must"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("must expects 1 argument")
			}
			v := args[0]
			if isError(v) {
				return v
			}
			if IsErrMap(v) {
				return newError("%s", FormatErr(v))
			}
			return v
		},
	}

	builtins["is_err"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return nativeBoolToBooleanObject(false)
			}
			return nativeBoolToBooleanObject(IsErrMap(args[0]) || isError(args[0]))
		},
	}

	builtins["builtins"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			pairs := make(map[string]object.Object, len(builtins))
			for k, v := range builtins {
				pairs[k] = v
			}
			return &object.Map{Pairs: pairs}
		},
	}

	builtins["vm_opcodes"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			return opcodeNameMap()
		},
	}

	builtins["vm_run"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("vm_run expects (chunk, env?)")
			}
			var envObj object.Object = NULL
			if len(args) == 2 {
				envObj = args[1]
			}
			return vmRun(args[0], envObj)
		},
	}
}
