package main

import (
	"bufio"
	"fmt"
	"neurolang/pkg/evaluator"
	"neurolang/pkg/mcp"
	"neurolang/pkg/object"
	"neurolang/pkg/tokenmetrics"
	"neurolang/pkg/tools"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.19.0"

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

	case "self":
		if len(os.Args) < 3 {
			fmt.Println("Usage: neurolang self <file.nl>")
			os.Exit(1)
		}
		runFile(os.Args[2])

	case "spec":
		printSpec()

	case "check":
		runCheck()

	case "mcp":
		mcp.Version = Version
		if err := mcp.Serve(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

	case "tools":
		printToolCatalog()

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
  neurolang run <file.nl>     Execute a script (boots std/compiler, then VM)
  neurolang self <file.nl>    Alias of run
  neurolang eval "<code>"     Evaluate a one-line expression
  neurolang repl              Launch interactive REPL session
  neurolang stats <file.nl>   Analyze token footprint and efficiency
  neurolang check             Rebuild stale std/*.nlc and run tests/*.nl
  neurolang spec              Print the dense AI language spec
  neurolang spec              Print the dense AI language spec
  neurolang tools             Compact tool catalog for models
  neurolang mcp               MCP stdio JSON-RPC (tools/list, tools/call)
  neurolang version           Show version

Native (no Go):
  gcc -O2 -o neurolang rt/nl.c rt/main.c


AI Combinators & Syntax Overview:
  |                           Pipeline (x | f)
  ?                           Filter stream (?(.age >= 18))
  @                           Map / Projection (@.name or @{id: .id})
  &                           Reduce / Fold (&((a, b) -> a + b))
  !                           External Tool / MCP call (!http.get(url))
  .                           Current context item in pipeline
  ->                          Lambda / Arrow function (x -> x * 2)
  match                       Pattern matching
  neurolang use "mod"         Load module, return export map
  for x in xs                 Iterate list / chars / map keys
  in                          Membership (list, map key, substring)

Examples:
  neurolang run examples/01_basics.nl
  neurolang eval "[1, 2, 3, 4] | ?(. > 2) | @(. * 10)"
  neurolang check
  neurolang spec
`, Version)
}

func bootGuest() *evaluator.Guest {
	g, err := evaluator.BootGuest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", evaluator.FormatErr(err))
		os.Exit(1)
	}
	return g
}

func runCheck() {
	g := bootGuest()
	v := evaluator.RunLangTests(g)
	if evaluator.IsErrMap(v) {
		fmt.Fprintf(os.Stderr, "%s\n", evaluator.FormatErr(v))
		os.Exit(1)
	}
	fmt.Println(v.Inspect())
}

func reportGuest(result object.Object, exitOnError bool) bool {
	if evaluator.IsErrMap(result) {
		fmt.Fprintf(os.Stderr, "%s\n", evaluator.FormatErr(result))
		if exitOnError {
			os.Exit(1)
		}
		return true
	}
	return false
}

func runFile(filename string) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %s\n", filename, err)
		os.Exit(1)
	}
	g := bootGuest()
	g.BindScript(filename)
	reportGuest(g.Eval(string(bytes), nil), true)
}

func printSpec() {
	candidates := []string{"SPEC_AI.md"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "SPEC_AI.md"))
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			fmt.Print(string(data))
			return
		}
	}
	fmt.Fprintf(os.Stderr, "SPEC_AI.md not found (run from the NeuroLang repo root)\n")
	os.Exit(1)
}

func printToolCatalog() {
	for _, s := range tools.Catalog() {
		sig := "!" + s.Name
		if len(s.Params) > 0 {
			sig += "(" + strings.Join(s.Params, ",") + ")"
		} else {
			sig += "()"
		}
		fmt.Printf("%-22s %s  %s\n", sig, s.Alias, s.Description)
	}
}

func evalCode(code string) {
	g := bootGuest()
	evaluated := g.Eval(code, nil)
	if reportGuest(evaluated, true) {
		return
	}
	if evaluated != nil && evaluated != evaluator.NULL {
		fmt.Println(evaluated.Inspect())
	}
}

func startRepl() {
	scanner := bufio.NewScanner(os.Stdin)
	g := bootGuest()
	env := g.NewEnv()

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

		evaluated := g.Eval(line, env)
		if reportGuest(evaluated, false) {
			continue
		}
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
