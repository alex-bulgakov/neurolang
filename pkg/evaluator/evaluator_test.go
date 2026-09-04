package evaluator

import (
	"fmt"
	"neurolang/pkg/object"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testGuest *Guest

func TestMain(m *testing.M) {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fmt.Fprintln(os.Stderr, "go.mod not found")
			os.Exit(1)
		}
		dir = parent
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	g, errObj := BootGuest()
	if g == nil {
		fmt.Fprintf(os.Stderr, "BootGuest: %s\n", FormatErr(errObj))
		os.Exit(1)
	}
	testGuest = g
	os.Exit(m.Run())
}

func TestNLSuite(t *testing.T) {
	v := RunLangTests(testGuest)
	if isError(v) || IsErrMap(v) {
		t.Fatalf("tests/*.nl: %s", FormatErr(v))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
		return false
	}
	return true
}

func inspectParity(v object.Object) string {
	if v == nil {
		return "<nil>"
	}
	return v.Inspect()
}

func TestStdBytecodeCache(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	stdDir, err := stdDirPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range stdNlcNames {
		if _, err := os.Stat(filepath.Join(stdDir, name+".nlc")); err != nil {
			t.Fatalf("expected cache file %s.nlc: %v", name, err)
		}
	}

	stdCompiler = nil
	g2, errObj := BootGuest()
	if g2 == nil {
		t.Fatalf("cached BootGuest: %s", FormatErr(errObj))
	}
	v := g2.Eval("5 + 2 * 10", nil)
	if isError(v) {
		t.Fatalf("cached eval: %s", v.Inspect())
	}
	testIntegerObject(t, v, 25)

	g3, errObj := BootGuest()
	if g3 == nil {
		t.Fatalf("BootGuest for use nlc: %s", FormatErr(errObj))
	}
	ident := g3.Eval(`
L = use "std/lexer"
L.tokenize("a = 1")[0].type
`, nil)
	if isError(ident) {
		t.Fatalf("use nlc: %s", ident.Inspect())
	}
	s, ok := ident.(*object.String)
	if !ok || s.Value != "IDENT" {
		t.Fatalf("use nlc: got %s", inspectParity(ident))
	}

	dir := t.TempDir()
	mod := filepath.Join(dir, "mod.nl")
	if err := os.WriteFile(mod, []byte("answer = 41 + 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	modSlash := filepath.ToSlash(mod)
	got := g3.Eval("M = use \""+modSlash+"\"\nM.answer", nil)
	if isError(got) {
		t.Fatalf("use temp module: %s", got.Inspect())
	}
	testIntegerObject(t, got, 42)
	nlc := nlcPath(mod)
	if _, err := os.Stat(nlc); err != nil {
		t.Fatalf("expected module nlc: %v", err)
	}
	if err := os.WriteFile(mod, []byte("!!!\n"), 0644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(nlc, later, later); err != nil {
		t.Fatal(err)
	}
	got = g3.Eval("M = use \""+modSlash+"\"\nM.answer", nil)
	if isError(got) {
		t.Fatalf("use stale source via nlc: %s", got.Inspect())
	}
	testIntegerObject(t, got, 42)
}

func TestBootRequiresNlc(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	stdDir, err := stdDirPath()
	if err != nil {
		t.Fatal(err)
	}
	prev := stdCompiler
	var moved []string
	for _, name := range stdNlcNames {
		p := filepath.Join(stdDir, name+".nlc")
		bak := p + ".bak"
		if err := os.Rename(p, bak); err != nil {
			t.Fatal(err)
		}
		moved = append(moved, p)
	}
	defer func() {
		for _, p := range moved {
			_ = os.Rename(p+".bak", p)
		}
		stdCompiler = prev
	}()
	stdCompiler = nil
	g, errObj := BootGuest()
	if g != nil {
		t.Fatal("expected BootGuest to fail without std/*.nlc")
	}
	if errObj == nil {
		t.Fatal("expected an error")
	}
}

func TestStaleNlcRebuildViaGuest(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	stdDir, err := stdDirPath()
	if err != nil {
		t.Fatal(err)
	}
	g, errObj := BootGuest()
	if g == nil {
		t.Fatalf("seed BootGuest: %s", FormatErr(errObj))
	}

	nl := filepath.Join(stdDir, "lexer.nl")
	nlc := filepath.Join(stdDir, "lexer.nlc")
	stSrc, err := os.Stat(nl)
	if err != nil {
		t.Fatal(err)
	}
	older := stSrc.ModTime().Add(-3 * time.Second)
	if err := os.Chtimes(nlc, older, older); err != nil {
		t.Fatal(err)
	}
	if cacheFresh(stdDir) {
		t.Fatal("expected stale cache after backdating lexer.nlc")
	}

	stdCompiler = nil
	g2, errObj := BootGuest()
	if g2 == nil {
		t.Fatalf("stale BootGuest: %s", FormatErr(errObj))
	}
	v := g2.Eval("5 + 2 * 10", nil)
	if isError(v) {
		t.Fatalf("rebuilt eval: %s", v.Inspect())
	}
	testIntegerObject(t, v, 25)
	if !cacheFresh(stdDir) {
		t.Fatal("expected guest rebuild to refresh std/*.nlc")
	}
}

func TestGuestUseModule(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	g, errObj := BootGuest()
	if g == nil {
		t.Fatalf("BootGuest: %s", FormatErr(errObj))
	}
	v := g.Eval(`
L = use "std/lexer"
toks = L.tokenize("a = 1")
toks[0].type
`, nil)
	if isError(v) {
		t.Fatalf("guest use lexer: %s", v.Inspect())
	}
	s, ok := v.(*object.String)
	if !ok || s.Value != "IDENT" {
		t.Fatalf("expected IDENT, got %s", inspectParity(v))
	}
}

func TestGuestRunAndREPL(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	g, errObj := BootGuest()
	if g == nil {
		t.Fatalf("BootGuest: %s", FormatErr(errObj))
	}

	v := g.Eval("5 + 2 * 10", nil)
	if isError(v) {
		t.Fatalf("guest arith: %s", v.Inspect())
	}
	testIntegerObject(t, v, 25)

	env := g.NewEnv()
	if isError(env) {
		t.Fatalf("NewEnv: %s", env.Inspect())
	}
	if v := g.Eval("x = 40", env); isError(v) {
		t.Fatalf("bind: %s", v.Inspect())
	}
	v = g.Eval("x + 2", env)
	if isError(v) {
		t.Fatalf("repl persist: %s", v.Inspect())
	}
	testIntegerObject(t, v, 42)
}
