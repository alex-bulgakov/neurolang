package evaluator

import (
	"neurolang/pkg/lexer"
	"neurolang/pkg/object"
	"neurolang/pkg/parser"
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
