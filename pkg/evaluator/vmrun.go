package evaluator

import (
	"fmt"
	"neurolang/pkg/object"
)

func opcodeNameMap() *object.Map {
	pairs := make(map[string]object.Object, len(opcodeNames))
	for i, n := range opcodeNames {
		pairs[n] = &object.Integer{Value: int64(i)}
	}
	return &object.Map{Pairs: pairs}
}

func chunkFromObject(obj object.Object) (*chunk, error) {
	m, ok := obj.(*object.Map)
	if !ok {
		return nil, fmt.Errorf("chunk must be a map")
	}
	ch := &chunk{}
	if code, ok := m.Pairs["code"].(*object.List); ok {
		for _, b := range code.Elements {
			switch n := b.(type) {
			case *object.Integer:
				ch.code = append(ch.code, byte(n.Value))
			case *object.Float:
				ch.code = append(ch.code, byte(n.Value))
			default:
				return nil, fmt.Errorf("code entry must be a number")
			}
		}
	}
	if names, ok := m.Pairs["names"].(*object.List); ok {
		for _, n := range names.Elements {
			if s, ok := n.(*object.String); ok {
				ch.names = append(ch.names, s.Value)
			} else {
				ch.names = append(ch.names, n.Inspect())
			}
		}
	}
	if consts, ok := m.Pairs["consts"].(*object.List); ok {
		for _, c := range consts.Elements {
			ch.consts = append(ch.consts, constFromObject(c))
		}
	}
	return ch, nil
}

func constFromObject(c object.Object) object.Object {
	m, ok := c.(*object.Map)
	if !ok {
		return c
	}
	if _, isBC := m.Pairs["__bc"]; !isBC && m.Pairs["code"] == nil {
		return c
	}
	inner, err := chunkFromObject(m)
	if err != nil {
		return c
	}
	var params []string
	if p, ok := m.Pairs["params"].(*object.List); ok {
		for _, x := range p.Elements {
			if s, ok := x.(*object.String); ok {
				params = append(params, s.Value)
			} else {
				params = append(params, x.Inspect())
			}
		}
	}
	return &vmClosure{ch: inner, params: params}
}

func envFromMap(m *object.Map) *object.Environment {
	env := object.NewEnvironment()
	if activeEnv != nil {
		env.File = activeEnv.File
		env.Dir = activeEnv.Dir
	}
	for k, v := range m.Pairs {
		env.Set(k, v)
	}
	return env
}

func copyEnvToMap(env *object.Environment, m *object.Map) {
	for k, v := range env.Bindings() {
		m.Pairs[k] = v
	}
}

func vmRun(chunkObj object.Object, envObj object.Object) object.Object {
	ch, err := chunkFromObject(chunkObj)
	if err != nil {
		return newError("vm_run: %s", err.Error())
	}
	env := object.NewEnvironment()
	var back *object.Map
	if m, ok := envObj.(*object.Map); ok {
		back = m
		env = envFromMap(m)
	} else if envObj != nil && envObj != NULL {
		return newError("vm_run: env must be a map")
	} else if activeEnv != nil {
		env.File = activeEnv.File
		env.Dir = activeEnv.Dir
	}
	res := run(ch, env)
	if back != nil {
		copyEnvToMap(env, back)
	}
	return res
}
