package evaluator

import (
	"neurolang/pkg/object"
	"path/filepath"
	"strings"
)

// stdCompiler is the export of std/compiler after the Go bootstrap load.
// Later use/load run sibling .nlc or compile with C.nl_parse/nl_compile.
var stdCompiler *object.Map

// Guest is a booted std/compiler. Existing std/*.nlc boot with vm_run;
// stale chunks are rebuilt by the guest compiler. Go loads std/compiler
// only when the cache files are missing.
type Guest struct {
	host *object.Environment
}

func BootGuest() (*Guest, object.Object) {
	host := object.NewEnvironment()
	stdDir, err := stdDirPath()
	if err == nil && cachePresent(stdDir) {
		if g := bootFromCache(host, stdDir); g != nil {
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
	}
	v := evalString(`C = use "std/compiler"`, host)
	if isError(v) {
		return nil, v
	}
	if err == nil {
		writeStdCache(host, stdDir)
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
	bc := loadFreshNlc(resolved)
	if bc == nil {
		bc = compileModuleChunk(src)
		if isError(bc) {
			return bc
		}
		if cache {
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
