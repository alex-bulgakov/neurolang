package parser

import (
	"neurolang/pkg/ast"
	"neurolang/pkg/lexer"
	"testing"
)

func TestAssignmentStatements(t *testing.T) {
	input := `
x = 42
pi = 3.14
msg = "hello"
flag = true
list = [1, 2, 3]
person = {name: "Alice", age: 30}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 6 {
		t.Fatalf("program.Statements does not contain 6 statements. got=%d",
			len(program.Statements))
	}

	expectedIdents := []string{"x", "pi", "msg", "flag", "list", "person"}
	for i, name := range expectedIdents {
		stmt, ok := program.Statements[i].(*ast.AssignStatement)
		if !ok {
			t.Fatalf("stmt not *ast.AssignStatement. got=%T", program.Statements[i])
		}
		if stmt.Name.Value != name {
			t.Fatalf("stmt.Name.Value not '%s'. got='%s'", name, stmt.Name.Value)
		}
	}
}

func TestPipelineParsing(t *testing.T) {
	input := `users | ?(.age >= 18) | @.name`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt is not *ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	// Should be: (users | ?(.age >= 18)) | @.name
	pipe, ok := stmt.Expression.(*ast.PipeExpression)
	if !ok {
		t.Fatalf("exp not *ast.PipeExpression. got=%T", stmt.Expression)
	}

	mapExp, ok := pipe.Right.(*ast.MapExpression)
	if !ok {
		t.Fatalf("pipe.Right not *ast.MapExpression. got=%T", pipe.Right)
	}

	dotExp, ok := mapExp.Transform.(*ast.DotExpression)
	if !ok {
		t.Fatalf("mapExp.Transform not *ast.DotExpression. got=%T", mapExp.Transform)
	}
	if dotExp.Field != "name" {
		t.Fatalf("dotExp.Field not 'name'. got=%s", dotExp.Field)
	}
}

func TestArrowFunctionParsing(t *testing.T) {
	input := `
double = x -> x * 2
add = (a, b) -> a + b
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}

	stmt1 := program.Statements[0].(*ast.AssignStatement)
	fn1, ok := stmt1.Value.(*ast.ArrowFunctionLiteral)
	if !ok {
		t.Fatalf("stmt1.Value is not *ast.ArrowFunctionLiteral. got=%T", stmt1.Value)
	}
	if len(fn1.Parameters) != 1 || fn1.Parameters[0].Value != "x" {
		t.Fatalf("fn1 parameters mismatch: %v", fn1.Parameters)
	}

	stmt2 := program.Statements[1].(*ast.AssignStatement)
	fn2, ok := stmt2.Value.(*ast.ArrowFunctionLiteral)
	if !ok {
		t.Fatalf("stmt2.Value is not *ast.ArrowFunctionLiteral. got=%T", stmt2.Value)
	}
	if len(fn2.Parameters) != 2 || fn2.Parameters[0].Value != "a" || fn2.Parameters[1].Value != "b" {
		t.Fatalf("fn2 parameters mismatch: %v", fn2.Parameters)
	}
}

func TestToolCallParsing(t *testing.T) {
	input := `!http.get("https://api.example.com/users")`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	toolCall, ok := stmt.Expression.(*ast.ToolCallExpression)
	if !ok {
		t.Fatalf("expected *ast.ToolCallExpression, got=%T", stmt.Expression)
	}

	if toolCall.ToolName != "http.get" {
		t.Fatalf("expected toolName 'http.get', got=%q", toolCall.ToolName)
	}

	if len(toolCall.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got=%d", len(toolCall.Arguments))
	}
}

func TestForInParsing(t *testing.T) {
	input := `for x in items { print(x) }`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("stmt not *ast.ForStatement. got=%T", program.Statements[0])
	}
	if stmt.Name.Value != "x" {
		t.Fatalf("loop var not 'x'. got=%s", stmt.Name.Value)
	}
}

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}
