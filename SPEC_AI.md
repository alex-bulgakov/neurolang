# NeuroLang Spec for Models

<!-- Inject as system primer. Dense by design: combinators over keywords. -->

## Grammar
```
P = S*
S = "return" E? | "while" E B | "for" ID "in" E B | "break" | "continue" | L "=" E | E
B = "{" S* "}"
E = Pipe
Pipe = Or ("|" ( "?" E | "@" E | "&" E | Call | ID ))*
Or = And ("||" And)*
And = In ("&&" In)*
In = Cmp ("in" Cmp)*
Cmp = Sum (("=="|"!="|"<"|"<="|">"|">=") Sum)*
Sum = Prod (("+"|"-") Prod)*
Prod = Pref (("*"|"/"|"%") Pref)*
Pref = ("-"|"!") Pref | Post
Post = Atom (("(" Args ")" | "[" E "]" | "." ID))*
Atom = NUM | STR | true | false | null | ID | List | Map | Lambda | "if" E B ("else" (B|"if"...))? | "match" E "{" (Pat "->" E)* "}" | "." ID? | "(" E ")" | "!" ID("." ID)* ("(" Args ")")?
Lambda = ID "->" E | "(" ID* ")" "->" E
List = "[" Args "]"
Map = "{" (ID|STR) ":" E ("," (ID|STR) ":" E)* "}"
```

Blocks `{S*}` are expressions (value = last S). Map vs block: `{k:v}` if first pair uses `:`, else block.

## Combinators (1 glyph = 1 op)
- `|` pipe: `x|f` == `f(x)`. Into `f(y)` becomes `f(x,y)`.
- `.` current item in `| ? @ for`. `.field` reads field. Bare `.` is the item.
- `?(pred)` filter. `@(expr)` map. `&(fn)` reduce. `!(tool)(...)` effect/MCP.
- `->` lambda. `in` membership (list/map-key/substring).
- `for x in xs { ... }` iterates list, string chars, or map keys. Sets `.`.

## Eval
Dynamic types: int float bool str null list map fn. `==` deep. `&&` `||` short-circuit.
Assign mutates ident / `obj.field` / `obj[k]`. `load("f.nl")` execs into current env.

## Builtins
`len keys values range append slice split join ord chr is_digit is_alpha is_space`
`int str float copy apply type print json parse_json builtins tool_call load`

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
Host Go runtime. Compiler subset lives in `std/{lexer,parser,evaluator,compiler}.nl`.
`nl_eval(code, env)` tokenizes+parses+evals. `env=null` => `copy(builtins())`.
CLI: `neurolang self file.nl` runs a program through that stack.

## Style for generation
No `def/function/class/import`. Prefer `| ? @` over loops. Short names. Omit types.
One statement per line. Parenthesize combinator predicates: `?(.x > 1)`.
