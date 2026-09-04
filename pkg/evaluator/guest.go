package evaluator

import (
	"neurolang/pkg/object"
	"path/filepath"
	"strings"
)

// stdCompiler is the export of std/compiler after boot from std/*.nlc.
var stdCompiler *object.Map

// Guest is a booted std/compiler. Boot is vm_run of committed std/*.nlc.
// Stale chunks are rebuilt by the guest compiler. There is no Go frontend.
type Guest struct {
	host *object.Environment
}

func BootGuest() (*Guest, object.Object) {
	stdDir, err := stdDirPath()
	if err != nil {
		return nil, newError("std/compiler not found: %s", err.Error())
	}
	if !cachePresent(stdDir) {
		return nil, newError("missing std/*.nlc; stage-0 bytecode is required to boot")
	}
	host := object.NewEnvironment()
	g := bootFromCache(host, stdDir)
	if g == nil {
		return nil, newError("failed to boot from std/*.nlc")
	}
	if !cacheFresh(stdDir) {
		writeStdCache(g.host, stdDir)
		if cacheFresh(stdDir) {
			stdCompiler = nil
			host2 := object.NewEnvironment()
			if g2 := bootFromCache(host2, stdDir); g2 != nil {
				return g2, nil
			}
		}
	}
	return g, nil
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
	return builtins["copy"].Fn(builtins["builtins"].Fn())
}

func (g *Guest) Eval(code string, env object.Object) object.Object {
	c, ok := g.host.Get("C")
	if !ok {
		return newError("compiler not booted")
	}
	cm, ok := c.(*object.Map)
	if !ok {
		return newError("compiler is not a map")
	}
	fn := cm.Pairs["nl_eval"]
	if fn == nil {
		return newError("compiler is missing nl_eval")
	}
	if env == nil {
		env = NULL
	}
	return applyFunction(fn, []object.Object{&object.String{Value: code}, env})
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
	result := evalModule(src, resolved, envMap, true)
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
	result := evalModule(src, resolved, envMap, true)
	env.File = prevFile
	env.Dir = prevDir
	return result
}

func evalModule(src, resolved string, envMap *object.Map, cache bool) object.Object {
	var bc object.Object
	if filepath.Base(resolved) != "compiler.nl" {
		bc = loadFreshNlc(resolved)
	}
	if bc == nil {
		bc = compileModuleChunk(src)
		if isError(bc) {
			return bc
		}
		if cache && filepath.Base(resolved) != "compiler.nl" {
			writeNlc(resolved, bc)
		}
	}
	return runModuleChunk(bc, resolved, envMap)
}

func runModuleChunk(bc object.Object, resolved string, envMap *object.Map) object.Object {
	prev := activeEnv
	tmp := object.NewEnvironment()
	tmp.File = resolved
	tmp.Dir = filepath.Dir(resolved)
	activeEnv = tmp
	defer func() { activeEnv = prev }()
	return vmRun(bc, envMap)
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
