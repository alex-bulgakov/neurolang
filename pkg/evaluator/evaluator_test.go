package evaluator

import (
	"neurolang/pkg/lexer"
	"neurolang/pkg/object"
	"neurolang/pkg/parser"
	"os"
	"path/filepath"
	"testing"
)

func testEval(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		panic(p.Errors()[0])
	}
	env := object.NewEnvironment()
	return Eval(program, env)
}

func TestIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestDataflowPipeline(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Filter
		{
			"[1, 2, 3, 4, 5] | ?(. > 3)",
			"[4, 5]",
		},
		// Map
		{
			"[1, 2, 3] | @(. * 10)",
			"[10, 20, 30]",
		},
		// Filter and Map chained
		{
			`
users = [
  {name: "Alice", age: 25, active: true},
  {name: "Bob", age: 17, active: true},
  {name: "Charlie", age: 30, active: false}
]

users | ?(.active && .age >= 18) | @.name
`,
			`["Alice"]`,
		},
		// Projection map transformation
		{
			`
items = [{id: 1, price: 100}, {id: 2, price: 200}]
items | @{id: .id, total: .price * 1.2}
`,
			`[{id: 1, total: 120}, {id: 2, total: 240}]`,
		},
		// Reduce
		{
			"[1, 2, 3, 4] | &((a, b) -> a + b)",
			"10",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if evaluated == nil {
			t.Fatalf("evaluated nil for input %s", tt.input)
		}
		if evaluated.Inspect() != tt.expected {
			t.Errorf("wrong result for input:\n%s\nexpected=%s, got=%s",
				tt.input, tt.expected, evaluated.Inspect())
		}
	}
}

func TestArrowFunctionsAndClosures(t *testing.T) {
	input := `
makeAdder = x -> (y -> x + y)
addTwo = makeAdder(2)
addTwo(3)
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 5)
}

func TestMatchExpression(t *testing.T) {
	input := `
classify = n -> match n {
  0 -> "zero"
  1 -> "one"
  _ -> "many"
}

[classify(0), classify(1), classify(99)]
`
	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("expected list, got=%T", evaluated)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 elements, got=%d", len(list.Elements))
	}
	if list.Elements[0].(*object.String).Value != "zero" ||
		list.Elements[1].(*object.String).Value != "one" ||
		list.Elements[2].(*object.String).Value != "many" {
		t.Fatalf("unexpected match results: %s", list.Inspect())
	}
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
		return false
	}
	return true
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

func TestWhileLoop(t *testing.T) {
	input := `
i = 0
total = 0
while i < 10 {
  i = i + 1
  if i == 5 {
    continue
  }
  if i > 8 {
    break
  }
  total = total + i
}
total
`
	evaluated := testEval(input)
	// i will be 1, 2, 3, 4, (skips 5), 6, 7, 8 (breaks at 9)
	// total = 1 + 2 + 3 + 4 + 6 + 7 + 8 = 31
	testIntegerObject(t, evaluated, 31)
}

func TestForInAndMembership(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`
acc = 0
for n in [1, 2, 3, 4] {
  acc = acc + n
}
acc
`,
			"10",
		},
		{
			`
s = ""
for ch in "ab" {
  s = s + ch
}
s
`,
			`"ab"`,
		},
		{
			`3 in [1, 2, 3]`,
			"true",
		},
		{
			`"name" in {name: "Ada", role: "agent"}`,
			"true",
		},
		{
			`"bc" in "abcd"`,
			"true",
		},
		{
			`9 in [1, 2, 3]`,
			"false",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if evaluated == nil {
			t.Fatalf("evaluated nil for input %s", tt.input)
		}
		if evaluated.Inspect() != tt.expected {
			t.Errorf("wrong result for input:\n%s\nexpected=%s, got=%s",
				tt.input, tt.expected, evaluated.Inspect())
		}
	}
}

func TestPropertyAssignment(t *testing.T) {
	input := `
obj = {count: 1}
obj.count = obj.count + 4
obj["tag"] = "ok"
obj.count + len(obj)
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 7)
}

func TestShortCircuitLogic(t *testing.T) {
	if v := testEval(`false && (1 / 0)`); v.Inspect() != "false" {
		t.Fatalf("expected false from short-circuit &&, got %s", v.Inspect())
	}
	if v := testEval(`true || (1 / 0)`); v.Inspect() != "true" {
		t.Fatalf("expected true from short-circuit ||, got %s", v.Inspect())
	}
}

func TestApplyCopyAndConversions(t *testing.T) {
	input := `
add = (a, b) -> a + b
n = apply(add, [10, 5])
s = str(n) + "x"
i = int("42")
src = {a: 1}
dup = copy(src)
dup.a = 9
[n, s, i, src.a, dup.a]
`
	evaluated := testEval(input)
	if evaluated.Inspect() != `[15, "15x", 42, 1, 9]` {
		t.Fatalf("unexpected result: %s", evaluated.Inspect())
	}
}

func TestMetaCircularSelfHost(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	input := `
C = use "std/compiler"
a = C.nl_eval("[10, 20, 30, 40] | ?(. > 15) | @(. * 2)", null)
b = C.nl_eval("square = n -> n * n\nsquare(8)", null)
c = C.nl_eval("acc = 0\nfor n in [1, 2, 3] {\n  acc = acc + n\n}\nacc", null)
d = C.nl_eval("u = {name: \"Ada\", active: true}\nif u.active { u.name } else { \"no\" }", null)
[a, b, c, d]
`
	evaluated := testEval(input)
	if isError(evaluated) {
		t.Fatalf("meta-circular eval error: %s", evaluated.Inspect())
	}
	got := evaluated.Inspect()
	want := `[[40, 60, 80], 64, 6, "Ada"]`
	if got != want {
		t.Fatalf("meta-circular mismatch:\nwant %s\ngot  %s", want, got)
	}
}

func TestUseModule(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	evaluated := testEval(`
L = use "std/lexer"
toks = L.tokenize("a = 1")
toks[0].type
`)
	if isError(evaluated) {
		t.Fatalf("use failed: %s", evaluated.Inspect())
	}
	s, ok := evaluated.(*object.String)
	if !ok || s.Value != "IDENT" {
		t.Fatalf("expected IDENT token type, got %s", evaluated.Inspect())
	}
}

func TestSelfHostedParseError(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	evaluated := testEval(`
C = use "std/compiler"
ast = C.nl_parse(")\n")
n = ast.statements[0]
[n.type, n.line, n.col]
`)
	if isError(evaluated) {
		t.Fatalf("nl_parse failed: %s", evaluated.Inspect())
	}
	got := evaluated.Inspect()
	if got != `["Err", 1, 1]` {
		t.Fatalf("expected Err at 1:1, got %s", got)
	}
}

func TestMustAndCoalesce(t *testing.T) {
	if v := testEval(`null ?? 7`); v.Inspect() != "7" {
		t.Fatalf("?? null: got %s", v.Inspect())
	}
	if v := testEval(`{err: "boom"} ?? 9`); v.Inspect() != "9" {
		t.Fatalf("?? err map: got %s", v.Inspect())
	}
	if v := testEval(`must(4)`); v.Inspect() != "4" {
		t.Fatalf("must ok: got %s", v.Inspect())
	}
	if v := testEval(`must({err: "x"})`); !isError(v) {
		t.Fatalf("must err should abort, got %s", v.Inspect())
	}
}

func TestDogfoodStdParse(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	evaluated := testEval(`
C = use "std/compiler"
lex_ast = C.nl_parse(!fs.read("std/lexer.nl"))
par_ast = C.nl_parse(!fs.read("std/parser.nl"))
lex_ok = len(lex_ast.statements) > 0 && lex_ast.statements[0].type != "Err"
par_ok = len(par_ast.statements) > 0 && par_ast.statements[0].type != "Err"
[lex_ok, par_ok, len(lex_ast.statements), len(par_ast.statements)]
`)
	if isError(evaluated) {
		t.Fatalf("dogfood parse failed: %s", evaluated.Inspect())
	}
	list, ok := evaluated.(*object.List)
	if !ok || len(list.Elements) != 4 {
		t.Fatalf("unexpected dogfood result: %s", evaluated.Inspect())
	}
	if !list.Elements[0].(*object.Boolean).Value || !list.Elements[1].(*object.Boolean).Value {
		t.Fatalf("std parse produced Err: %s", evaluated.Inspect())
	}
}

func TestNLCompileVMRun(t *testing.T) {
	root := repoRoot(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	evaluated := testEval(`
C = use "std/compiler"
bc = C.nl_compile(C.nl_parse("acc = 0\nfor n in [1, 2, 3] {\n  acc = acc + n\n}\nacc"))
vm_run(bc, copy(builtins()))
`)
	if isError(evaluated) {
		t.Fatalf("nl_compile/vm_run failed: %s", evaluated.Inspect())
	}
	testIntegerObject(t, evaluated, 6)
}

func TestListConcatAndSlice(t *testing.T) {
	input := `
a = [1, 2]
b = [3, 4]
c = a + b
s = slice(c, 1, 3)
s
`
	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("expected list, got=%T", evaluated)
	}
	if len(list.Elements) != 2 || list.Elements[0].(*object.Integer).Value != 2 || list.Elements[1].(*object.Integer).Value != 3 {
		t.Fatalf("unexpected slice result: %s", list.Inspect())
	}
}

func TestCharBuiltins(t *testing.T) {
	input := `
c = chr(65)
code = ord("B")
isD = is_digit("7")
isA = is_alpha("_")
[c, code, isD, isA]
`
	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("expected list, got=%T", evaluated)
	}
	if list.Elements[0].(*object.String).Value != "A" ||
		list.Elements[1].(*object.Integer).Value != 66 ||
		list.Elements[2].(*object.Boolean).Value != true ||
		list.Elements[3].(*object.Boolean).Value != true {
		t.Fatalf("unexpected char result: %s", list.Inspect())
	}
}
