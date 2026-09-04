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

func stdDirPath() (string, error) {
	resolved, err := ResolveModulePath("std/compiler", nil)
	if err != nil {
		return "", err
	}
	return filepath.Dir(resolved), nil
}

func cacheFresh(stdDir string) bool {
	var newestSrc time.Time
	for _, name := range append(stdCacheNames, "compiler") {
		st, err := os.Stat(filepath.Join(stdDir, name+".nl"))
		if err != nil {
			return false
		}
		if st.ModTime().After(newestSrc) {
			newestSrc = st.ModTime()
		}
	}
	var oldestNlc time.Time
	for i, name := range stdCacheNames {
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
	chunks := make([]object.Object, 0, 3)
	for _, name := range stdCacheNames {
		data, err := os.ReadFile(filepath.Join(stdDir, name+".nlc"))
		if err != nil {
			return nil
		}
		obj, err := decodeChunkJSON(data)
		if err != nil {
			return nil
		}
		chunks = append(chunks, obj)
	}

	prev := activeEnv
	host.File = filepath.Join(stdDir, "compiler.nl")
	host.Dir = stdDir
	activeEnv = host
	defer func() { activeEnv = prev }()

	empty := &object.Map{Pairs: map[string]object.Object{}}
	L := vmRun(chunks[0], empty)
	if isError(L) {
		return nil
	}
	P := vmRun(chunks[1], &object.Map{Pairs: map[string]object.Object{}})
	if isError(P) {
		return nil
	}
	K := vmRun(chunks[2], &object.Map{Pairs: map[string]object.Object{}})
	if isError(K) {
		return nil
	}

	host.Set("L", L)
	host.Set("P", P)
	host.Set("K", K)

	src, err := os.ReadFile(filepath.Join(stdDir, "compiler.nl"))
	if err != nil {
		return nil
	}
	v := evalString(stripStdUses(string(src)), host)
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
	for _, name := range stdCacheNames {
		data, err := os.ReadFile(filepath.Join(stdDir, name+".nl"))
		if err != nil {
			return
		}
		ast := applyFunction(parse, []object.Object{&object.String{Value: string(data)}})
		if isError(ast) {
			return
		}
		bc := applyFunction(compile, []object.Object{ast})
		if isError(bc) {
			return
		}
		raw, err := encodeChunkJSON(bc)
		if err != nil {
			return
		}
		_ = os.WriteFile(filepath.Join(stdDir, name+".nlc"), raw, 0644)
	}
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
