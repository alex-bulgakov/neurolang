package evaluator

import (
	"os"

	"neurolang/pkg/object"
)

func evalNLFile(g *Guest, path string) object.Object {
	data, err := os.ReadFile(path)
	if err != nil {
		return newError("%s", err.Error())
	}
	prevF, prevD := g.host.File, g.host.Dir
	g.BindScript(path)
	defer func() {
		g.host.File = prevF
		g.host.Dir = prevD
	}()
	return g.Eval(string(data), nil)
}

// RunLangTests executes tests/all.nl (must succeed) and tests/err.nl (must fail).
func RunLangTests(g *Guest) object.Object {
	okPath, err := ResolveModulePath("tests/all", nil)
	if err != nil {
		return newError("tests/all.nl not found")
	}
	v := evalNLFile(g, okPath)
	if isError(v) || IsErrMap(v) {
		return v
	}
	errPath, err := ResolveModulePath("tests/err", nil)
	if err != nil {
		return newError("tests/err.nl not found")
	}
	v = evalNLFile(g, errPath)
	if !isError(v) && !IsErrMap(v) {
		return newError("tests/err.nl should fail, got %s", v.Inspect())
	}
	return &object.String{Value: "ok"}
}
