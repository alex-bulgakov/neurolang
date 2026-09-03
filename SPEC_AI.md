# NeuroLang Specification for AI Models (Dense Spec)

<!-- Token Footprint: ~380 BPE tokens. Inject into LLM system prompt as context primer. -->

## 1. Syntax & Core Grammar (EBNF)
```ebnf
Program    ::= Statement*
Statement  ::= Assign | Return | While | Expr
Assign     ::= IDENT "=" Expr
While      ::= "while" Expr Block
Block      ::= "{" Statement* "}"

Expr       ::= PipeExpr
PipeExpr   ::= LogicExpr ("|" (Filter | Map | Reduce | Call | IDENT))*
Filter     ::= "?" Expr
Map        ::= "@" Expr
Reduce     ::= "&" Expr
ToolCall   ::= "!" IDENT ("." IDENT)* ("(" (Expr ("," Expr)*)? ")")?

LogicExpr  ::= CompExpr (("&&" | "||") CompExpr)*
CompExpr   ::= MathExpr (("==" | "!=" | "<" | "<=" | ">" | ">=") MathExpr)*
MathExpr   ::= Term (("+" | "-") Term)*
Term       ::= Factor (("*" | "/" | "%") Factor)*
Factor     ::= ("-" | "!")? Primary
Primary    ::= INT | FLOAT | STRING | BOOL | "null" | List | MapLit | Lambda | Match | Dot | "(" Expr ")"

Dot        ::= "." IDENT?
Lambda     ::= ("(" IDENT ("," IDENT)* ")" | IDENT) "->" Expr
Match      ::= "match" Expr "{" (Pattern "->" Expr ("," | ";")?)* "}"
List       ::= "[" (Expr ("," Expr)*)? "]"
MapLit     ::= "{" (Key ":" Expr ("," Key ":" Expr)*)? "}"
Key        ::= IDENT | STRING
```

## 2. Semantics of Combinators
- `|` (Pipe): Streams output of left into right. `data | f` == `f(data)`.
- `.` (Dot): The implicit current item inside stream operations.
  - Solitary `.`: Current item itself.
  - `.field`: Field access on current item (`item["field"]`).
- `?(condition)`: Stream filter. Retains items where condition with `.` is truthy.
- `@(transform)`: Stream map. Transforms each item using `.` or expression.
- `&(reducer)`: Stream fold/reduce using `(acc, item) -> ...`.
- `!tool.name(args)`: Side effect / external tool / MCP call.

## 3. Built-in Primitives
- **Collections**: `len(x)`, `keys(m)`, `values(m)`, `range(start, end)`, `append(list, elem)`, `slice(seq, start, end)`
- **Strings/Bytes**: `ord(char)`, `chr(int)`, `is_digit(char)`, `is_alpha(char)`, `is_space(char)`, `split(str, sep)`, `join(list, sep)`
- **Serialization**: `json(obj)`, `parse_json(str)`
- **Modules**: `load("file.nl")` (executes file in current environment)
- **Effects**: `!fs.read(path)`, `!fs.write(path, data)`, `!fs.list(path)`, `!http.get(url)`, `!http.post(url, body)`, `!time.now()`, `!time.sleep(ms)`

## 4. Canonical Patterns
```nl
# Filter & Map:
active_names = users | ?(.is_active && .age >= 18) | @.name

# Object Projection:
summaries = orders | @{id: .id, total: .qty * .price}

# Sum / Aggregation:
total = [10, 20, 30] | &((a, b) -> a + b)

# Tool Pipelines:
logs = "app.log" | !fs.read | split("\n") | ?(. != "")
```
