package main

import (
	"bufio"
	"fmt"
	"neurolang/pkg/evaluator"
	"neurolang/pkg/lexer"
	"neurolang/pkg/object"
	"neurolang/pkg/parser"
	"neurolang/pkg/tokenmetrics"
	"os"
	"strings"
)

const Version = "0.1.0-alpha"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	command := os.Args[1]

	switch command {
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: neurolang run <file.nl>")
			os.Exit(1)
		}
		runFile(os.Args[2])

	case "eval":
		if len(os.Args) < 3 {
			fmt.Println("Usage: neurolang eval \"<code>\"")
			os.Exit(1)
		}
		evalCode(os.Args[2])

	case "repl":
		startRepl()

	case "stats":
		if len(os.Args) < 3 {
			fmt.Println("Usage: neurolang stats <file.nl> [optional_compare.py]")
			os.Exit(1)
		}
		showStats(os.Args[2])

	case "version", "-v", "--version":
		fmt.Printf("NeuroLang v%s (AI-Native Runtime)\n", Version)

	case "help", "-h", "--help":
		printHelp()

	default:
		// If argument ends with .nl, run it directly
		if strings.HasSuffix(command, ".nl") {
			runFile(command)
		} else {
			fmt.Printf("Unknown command: %s\n", command)
			printHelp()
			os.Exit(1)
		}
	}
}

func printHelp() {
	fmt.Printf(`NeuroLang v%s - AI-Native Programming Language & Runtime

Usage:
  neurolang run <file.nl>     Execute a NeuroLang script
  neurolang eval "<code>"     Evaluate a one-line expression
  neurolang repl              Launch interactive REPL session
  neurolang stats <file.nl>   Analyze token footprint and efficiency
  neurolang version           Show version

AI Combinators & Syntax Overview:
  |                           Pipeline (x | f)
  ?                           Filter stream (?(.age >= 18))
  @                           Map / Projection (@.name or @{id: .id})
  &                           Reduce / Fold (&((a, b) -> a + b))
  !                           External Tool / MCP call (!http.get(url))
  .                           Current context item in pipeline
  ->                          Lambda / Arrow function (x -> x * 2)
  match                       Pattern matching

Examples:
  neurolang run examples/01_basics.nl
  neurolang eval "[1, 2, 3, 4] | ?(. > 2) | @(. * 10)"
`, Version)
}

func runFile(filename string) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %s\n", filename, err)
		os.Exit(1)
	}

	code := string(bytes)
	env := object.NewEnvironment()
	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintf(os.Stderr, "Parse errors in %s:\n", filename)
		for _, msg := range p.Errors() {
			fmt.Fprintf(os.Stderr, "  - %s\n", msg)
		}
		os.Exit(1)
	}

	evaluated := evaluator.Eval(program, env)
	if evaluated != nil && evaluated.Type() == object.ERROR_OBJ {
		fmt.Fprintf(os.Stderr, "%s\n", evaluated.Inspect())
		os.Exit(1)
	}
}

func evalCode(code string) {
	env := object.NewEnvironment()
	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintf(os.Stderr, "Parse errors:\n")
		for _, msg := range p.Errors() {
			fmt.Fprintf(os.Stderr, "  - %s\n", msg)
		}
		os.Exit(1)
	}

	evaluated := evaluator.Eval(program, env)
	if evaluated != nil && evaluated != evaluator.NULL {
		fmt.Println(evaluated.Inspect())
	}
}

func startRepl() {
	scanner := bufio.NewScanner(os.Stdin)
	env := object.NewEnvironment()

	fmt.Printf("NeuroLang REPL v%s\nType 'exit' or Ctrl+C to quit.\n\n", Version)

	for {
		fmt.Print("nl> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "exit" || line == "quit" {
			break
		}
		if line == "" {
			continue
		}

		l := lexer.New(line)
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			for _, msg := range p.Errors() {
				fmt.Printf("Error: %s\n", msg)
			}
			continue
		}

		evaluated := evaluator.Eval(program, env)
		if evaluated != nil && evaluated != evaluator.NULL {
			fmt.Println(evaluated.Inspect())
		}
	}
}

func showStats(filename string) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	code := string(bytes)
	nlMetrics := tokenmetrics.Analyze(code, "NeuroLang")

	fmt.Println("==================================================")
	fmt.Printf(" NeuroLang Token & Context Footprint Analysis\n")
	fmt.Println("==================================================")
	fmt.Printf("File:            %s\n", filename)
	fmt.Printf("Characters:      %d\n", nlMetrics.CharCount)
	fmt.Printf("Lines:           %d\n", nlMetrics.LineCount)
	fmt.Printf("Estimated BPE:   %d tokens\n\n", nlMetrics.TokenCount)

	// If a second argument is provided (e.g. equivalent Python file), compare
	if len(os.Args) >= 4 {
		cmpFile := os.Args[3]
		cmpBytes, err := os.ReadFile(cmpFile)
		if err == nil {
			cmpMetrics := tokenmetrics.Analyze(string(cmpBytes), "Comparison")
			fmt.Printf("Comparison File: %s\n", cmpFile)
			fmt.Printf("Comparison BPE:  %d tokens\n", cmpMetrics.TokenCount)

			savedTokens := cmpMetrics.TokenCount - nlMetrics.TokenCount
			percent := float64(savedTokens) / float64(cmpMetrics.TokenCount) * 100.0
			fmt.Printf("\n--> Saved %d tokens (%.1f%% context reduction!)\n", savedTokens, percent)
		}
	}
	fmt.Println("==================================================")
}
