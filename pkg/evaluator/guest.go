package evaluator

import (
	"neurolang/pkg/object"
	"path/filepath"
)

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
