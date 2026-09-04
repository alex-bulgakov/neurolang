# NeuroLang Spec for Models

<!-- Inject as system primer. Dense by design: combinators over keywords. -->

## Grammar
```
P = S*
S = "return" E? | "while" E B | "for" ID "in" E B | "break" | "continue" | L "=" E | E
B = "{" S* "}"
E = Coalesce
Coalesce = Pipe ("??" Pipe)*
Pipe = Or ("|" ( "?" E | "@" E | "&" E | Call | ID ))*
Or = And ("||" And)*
And = In ("&&" In)*
In = Cmp ("in" Cmp)*
Cmp = Sum (("=="|"!="|"<"|"<="|">"|">=") Sum)*
Sum = Prod (("+"|"-") Prod)*
Prod = Pref (("*"|"/"|"%") Pref)*
Pref = ("-"|"!") Pref | Post
Post = Atom (("(" Args ")" | "[" E "]" | "." ID))*
Atom = NUM | STR | true | false | null | ID | List | Map | Lambda | "if" E B ("else" (B|"if"...))? | "match" E "{" (Pat "->" E)* "}" | "." ID? | "(" E ")" | "!" ID("." ID)* ("(" Args ")")? | "use" E
Lambda = ID "->" E | "(" ID* ")" "->" E
List = "[" Args "]"
Map = "{" (ID|STR) ":" E ("," (ID|STR) ":" E)* "}"
```

Blocks `{S*}` are expressions (value = last S). Map vs block: `{k:v}` if first pair uses `:`, else block.

## Combinators (1 glyph = 1 op)
- `|` pipe: `x|f` == `f(x)`. Into `f(y)` becomes `f(x,y)`.
- `.` current item in `| ? @ for`. `.field` reads field. Bare `.` is the item.
- `?(pred)` filter. `@(expr)` map. `&(fn)` reduce. `!(tool)(...)` effect/MCP.
- `->` lambda. `in` membership. `??` default if null/err (`x ?? 0`). `must(v)` aborts on `{err:...}`.
- `for x in xs { ... }` iterates list, string chars, or map keys. Sets `.`.

## Eval
Dynamic types: int float bool str null list map fn. `==` deep. `&&` `||` short-circuit. Infix does not cross newline except `| ?? && || + - * /`.
Assign mutates ident / `obj.field` / `obj[k]`. `use "std/lexer"` loads a module in a fresh env and returns its export map (`L.tokenize`); after boot, sibling `.nlc` is `vm_run` when newer than source. `load("f.nl")` still dumps bindings into the current env.

## Builtins
`len keys values range append slice split join ord chr is_digit is_alpha is_space`
`int str float copy apply type print json parse_json builtins tool_call load use must is_err vm_run vm_opcodes`

## Tools
`!http.get/post !fs.list/read/write !env.get !time.now/sleep`

## Patterns
```
users | ?(.active && .age >= 18) | @.name
orders | @{id:.id, total:.qty*.price} | ?(.total > 100)
xs | &((a,b) -> a+b)
for ch in src { if is_digit(ch) { n = n*10 + (ord(ch)-48) } }
!fs.read("x.json") | @.items | ?(.ok)
match role { "admin" -> "H", _ -> "L" }
```

## Self-host
Host is a stack VM. Compiler subset lives in `std/{lexer,parser,compile,compiler}.nl` (`evaluator.nl` is a debug tree-walk).
`C = use "std/compiler"` then `C.nl_eval(code, env)` = parse + compile + `vm_run`. Host `Eval` and guest `nl_eval` agree on the language corpus. `env=null` => `copy(builtins())`. Guest parse+compile of `std/{lexer,parser,compile,compiler}.nl` yields bytecode that runs on the VM.
Last map in a module file is the export; otherwise all top-level names. After `std/compiler` is booted, `use`/`load` `vm_run` a sibling `.nlc` when it is newer than the source, otherwise compile with `C.nl_parse`/`nl_compile` and write `.nlc`. Committed `std/{lexer,parser,compile,compiler}.nlc` boots with `vm_run` only (`compiler.nlc` gets `L`/`P`/`K`). Stale chunks are rebuilt by the guest compiler; Go loads `std/compiler` only if the `.nlc` files are missing.
CLI: `neurolang run` / `eval` / `repl` boot `std/compiler` then `C.nl_eval` (`self` is an alias of `run`). Fresh `std/*.nlc` skips the Go load. `neurolang tools` / `neurolang mcp` expose the tool registry.
Parse errors are `{type:"Err", msg, line, col}`. `!ident` is always a tool; boolean not uses `!(expr)` or `x == false`.

## Style for generation
No `def/function/class/import`. Prefer `| ? @` over loops. Short names. Omit types.
One statement per line. Parenthesize combinator predicates: `?(.x > 1)`.
