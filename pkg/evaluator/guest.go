package evaluator

import (
	"neurolang/pkg/object"
	"path/filepath"
	"strings"
)

// stdCompiler is the export of std/compiler after the Go bootstrap load.
// Later use/load compile modules with C.nl_eval instead of the Go frontend.
var stdCompiler *object.Map

// Guest is a booted std/compiler. The Go frontend loads std once;
// user code is parsed and compiled by NeuroLang itself.
type Guest struct {
	host *object.Environment
}

func BootGuest() (*Guest, object.Object) {
	host := object.NewEnvironment()
	v := evalString(`C = use "std/compiler"`, host)
	if isError(v) {
		return nil, v
	}
	return &Guest{host: host}, nil
}

func (g *Guest) BindScript(path string) {
	if g == nil || path == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	g.host.File = abs
	g.host.Dir = filepath.Dir(abs)
}

func (g *Guest) NewEnv() object.Object {
	return evalString(`copy(builtins())`, g.host)
}

func (g *Guest) Eval(code string, env object.Object) object.Object {
	g.host.Set("__src", &object.String{Value: code})
	if env == nil || env == NULL {
		return evalString(`C.nl_eval(__src, null)`, g.host)
	}
	g.host.Set("__env", env)
	return evalString(`C.nl_eval(__src, __env)`, g.host)
}

func evalString(code string, env *object.Environment) object.Object {
	prog, errObj := parseSource(code, "nl")
	if errObj != nil {
		return errObj
	}
	return Eval(prog, env)
}

func rememberCompiler(m *object.Map) {
	if stdCompiler != nil || m == nil {
		return
	}
	if _, ok := m.Pairs["nl_eval"]; !ok {
		return
	}
	if _, ok := m.Pairs["nl_compile"]; !ok {
		return
	}
	stdCompiler = m
}

func useViaStd(src, resolved string) object.Object {
	envMap := &object.Map{Pairs: make(map[string]object.Object)}
	result := callStdEval(src, envMap, resolved)
	if isError(result) {
		return result
	}
	if IsErrMap(result) {
		return newError("%s", FormatErr(result))
	}
	if m, ok := result.(*object.Map); ok {
		return m
	}
	return exportMap(envMap)
}

func loadViaStd(src, resolved string, env *object.Environment) object.Object {
	envMap := &object.Map{Pairs: env.Bindings()}
	prevFile, prevDir := env.File, env.Dir
	env.File = resolved
	env.Dir = filepath.Dir(resolved)
	result := callStdEval(src, envMap, resolved)
	env.File = prevFile
	env.Dir = prevDir
	return result
}

func callStdEval(src string, envMap *object.Map, resolved string) object.Object {
	fn := stdCompiler.Pairs["nl_eval"]
	prev := activeEnv
	tmp := object.NewEnvironment()
	tmp.File = resolved
	tmp.Dir = filepath.Dir(resolved)
	activeEnv = tmp
	defer func() { activeEnv = prev }()
	return applyFunction(fn, []object.Object{&object.String{Value: src}, envMap})
}

func exportMap(m *object.Map) *object.Map {
	pairs := make(map[string]object.Object)
	for k, v := range m.Pairs {
		if strings.HasPrefix(k, "__") {
			continue
		}
		pairs[k] = v
	}
	return &object.Map{Pairs: pairs}
}
