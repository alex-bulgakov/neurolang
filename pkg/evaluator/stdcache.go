package evaluator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"neurolang/pkg/object"
)

var stdCacheNames = []string{"lexer", "parser", "compile"}
var stdNlcNames = []string{"lexer", "parser", "compile", "compiler"}

func stdDirPath() (string, error) {
	resolved, err := ResolveModulePath("std/compiler", nil)
	if err != nil {
		return "", err
	}
	return filepath.Dir(resolved), nil
}

func cacheFresh(stdDir string) bool {
	var newestSrc time.Time
	for _, name := range stdNlcNames {
		st, err := os.Stat(filepath.Join(stdDir, name+".nl"))
		if err != nil {
			return false
		}
		if st.ModTime().After(newestSrc) {
			newestSrc = st.ModTime()
		}
	}
	var oldestNlc time.Time
	for i, name := range stdNlcNames {
		st, err := os.Stat(filepath.Join(stdDir, name+".nlc"))
		if err != nil {
			return false
		}
		if i == 0 || st.ModTime().Before(oldestNlc) {
			oldestNlc = st.ModTime()
		}
	}
	return !oldestNlc.Before(newestSrc)
}

func bootFromCache(host *object.Environment, stdDir string) *Guest {
	prev := activeEnv
	host.File = filepath.Join(stdDir, "compiler.nl")
	host.Dir = stdDir
	activeEnv = host
	defer func() { activeEnv = prev }()

	parts := make([]object.Object, 0, 3)
	for _, name := range stdCacheNames {
		bc := readNlc(filepath.Join(stdDir, name+".nlc"))
		if bc == nil {
			return nil
		}
		v := vmRun(bc, &object.Map{Pairs: map[string]object.Object{}})
		if isError(v) {
			return nil
		}
		parts = append(parts, v)
	}
	L, P, K := parts[0], parts[1], parts[2]
	host.Set("L", L)
	host.Set("P", P)
	host.Set("K", K)

	comp := readNlc(filepath.Join(stdDir, "compiler.nlc"))
	if comp == nil {
		return nil
	}
	v := vmRun(comp, &object.Map{Pairs: map[string]object.Object{"L": L, "P": P, "K": K}})
	if isError(v) {
		return nil
	}
	m, ok := v.(*object.Map)
	if !ok {
		return nil
	}
	host.Set("C", m)
	rememberCompiler(m)
	return &Guest{host: host}
}

func stripStdUses(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "L = use ") || strings.HasPrefix(t, "P = use ") || strings.HasPrefix(t, "K = use ") {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func writeStdCache(host *object.Environment, stdDir string) {
	c, ok := host.Get("C")
	if !ok {
		return
	}
	cm, ok := c.(*object.Map)
	if !ok {
		return
	}
	parse := cm.Pairs["nl_parse"]
	compile := cm.Pairs["nl_compile"]
	if parse == nil || compile == nil {
		return
	}
	for _, name := range stdNlcNames {
		data, err := os.ReadFile(filepath.Join(stdDir, name+".nl"))
		if err != nil {
			return
		}
		src := string(data)
		if name == "compiler" {
			src = stripStdUses(src)
		}
		bc := compileModuleChunk(src)
		if isError(bc) {
			return
		}
		writeNlc(filepath.Join(stdDir, name+".nl"), bc)
	}
}

func nlcPath(nlPath string) string {
	if strings.HasSuffix(nlPath, ".nl") {
		return strings.TrimSuffix(nlPath, ".nl") + ".nlc"
	}
	return nlPath + ".nlc"
}

func loadFreshNlc(nlPath string) object.Object {
	nlc := nlcPath(nlPath)
	stSrc, err1 := os.Stat(nlPath)
	stNlc, err2 := os.Stat(nlc)
	if err1 != nil || err2 != nil {
		return nil
	}
	if stNlc.ModTime().Before(stSrc.ModTime()) {
		return nil
	}
	return readNlc(nlc)
}

func readNlc(path string) object.Object {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	obj, err := decodeChunkJSON(data)
	if err != nil {
		return nil
	}
	return obj
}

func writeNlc(nlPath string, bc object.Object) {
	raw, err := encodeChunkJSON(bc)
	if err != nil {
		return
	}
	_ = os.WriteFile(nlcPath(nlPath), raw, 0644)
}

func compileModuleChunk(src string) object.Object {
	if stdCompiler == nil {
		return newError("std compiler is not booted")
	}
	parse := stdCompiler.Pairs["nl_parse"]
	compile := stdCompiler.Pairs["nl_compile"]
	if parse == nil || compile == nil {
		return newError("std compiler is missing nl_parse/nl_compile")
	}
	ast := applyFunction(parse, []object.Object{&object.String{Value: src}})
	if isError(ast) {
		return ast
	}
	if errObj := moduleParseErr(ast); errObj != nil {
		return errObj
	}
	bc := applyFunction(compile, []object.Object{ast})
	if isError(bc) {
		return bc
	}
	return bc
}

func moduleParseErr(ast object.Object) object.Object {
	m, ok := ast.(*object.Map)
	if !ok {
		return nil
	}
	stmts, ok := m.Pairs["statements"].(*object.List)
	if !ok || len(stmts.Elements) == 0 {
		return nil
	}
	first, ok := stmts.Elements[0].(*object.Map)
	if !ok {
		return nil
	}
	t, _ := first.Pairs["type"].(*object.String)
	if t == nil || t.Value != "Err" {
		return nil
	}
	msg := ""
	if s, ok := first.Pairs["msg"].(*object.String); ok {
		msg = s.Value
	}
	line, col := 0, 0
	if n, ok := first.Pairs["line"].(*object.Integer); ok {
		line = int(n.Value)
	}
	if n, ok := first.Pairs["col"].(*object.Integer); ok {
		col = int(n.Value)
	}
	return newError("%s", FormatErr(ErrMap(msg, line, col)))
}

func encodeChunkJSON(obj object.Object) ([]byte, error) {
	return json.Marshal(obj.ToInterface())
}

func decodeChunkJSON(data []byte) (object.Object, error) {
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return wholeFloatsToInts(object.FromGoValue(parsed)), nil
}

func wholeFloatsToInts(o object.Object) object.Object {
	switch v := o.(type) {
	case *object.Float:
		n := int64(v.Value)
		if v.Value == float64(n) {
			return &object.Integer{Value: n}
		}
	case *object.List:
		for i, e := range v.Elements {
			v.Elements[i] = wholeFloatsToInts(e)
		}
	case *object.Map:
		for k, e := range v.Pairs {
			v.Pairs[k] = wholeFloatsToInts(e)
		}
	}
	return o
}
