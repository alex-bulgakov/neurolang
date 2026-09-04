# NeuroLang Language Specification

NeuroLang (NL) is an AI-native dataflow language: text that is cheap to generate and cheap to read for transformers, then executed by an independent runtime.

The Go package in this repository is the **host** (stack VM, builtins, tools). The compiler lives in `std/` and ships as committed `std/*.nlc`. Language changes go in `std/`; a stale cache is rebuilt by the guest compiler. There is no Go lexer, parser, or bytecode compiler.

## 1. Why this shape

Transformers pay per token. Keywords, braces-as-ceremony, and long identifiers burn context without adding semantics. Binary encodings are worse: BPE splits hex/Base64, and attention cannot keep byte offsets consistent.

NL therefore:

1. Keeps a **tiny glyph set** whose operators usually occupy one BPE token (`| ? @ & ! . ->`).
2. Makes **pipelines** the default control structure (agent work is almost always transform-in → transform-out).
3. Treats **tools/MCP** as syntax (`!http.get`), not imported SDKs.
4. Stays **textual** so a human or a reviewer-agent can audit a trace without a decompiler.

## 2. Lexical structure

Comments: `# ...` or `// ...` to end of line.

Strings: `"..."` or `'...'` with escapes `\n \t \r \\ \" \'`.

Numbers: integers and floats (`3.14`).

Identifiers: `[A-Za-z_$][A-Za-z0-9_]*`.

Keywords: `true false null if else match while for in break continue return use`.

Two-character operators: `== != <= >= && || -> ??`.

One-character operators / combinators: `= + - * / % | ? @ & ! . , : ; ( ) [ ] { }`.

## 3. Programs and statements

A program is a sequence of statements. The value of a program is the value of its last statement (unless `return` unwraps earlier).

| Form | Meaning |
|---|---|
| `x = E` | bind / assign |
| `obj.f = E` / `obj[k] = E` | mutate map field / index |
| `return E` | return from function or block |
| `while E { S* }` | loop |
| `for x in E { S* }` | iterate list, string (chars), or map (keys) |
| `break` / `continue` | loop control |
| `E` | expression statement |

`for` also binds `.` to the current element, so pipeline-style bodies work inside loops.

## 4. Expressions

Precedence, tightest last:

`??` → `\|` → `\|\|` → `&&` → `in` / comparisons → `+ -` → `* / %` → prefix `- !` → call / index / `.field`

### 4.1 Dataflow combinators

- `x | f` calls `f(x)`. `x | f(y)` calls `f(x, y)`.
- `xs | ?(pred)` keeps elements where `pred` is truthy with `.` bound to the element.
- `xs | @(expr)` maps each element.
- `xs | &(fn)` folds with a two-argument function.
- `.` is the current element; `.a.b` walks fields.
- `!tool.name(args)` is an effect. Piped form: `x | !fs.read`.

Always parenthesize non-trivial predicates: `?(.amount >= 100)`.

### 4.2 Functions and match

```
square = n -> n * n
add = (a, b) -> a + b
priority = match role { "admin" -> "H", _ -> "L" }
```

A block `{ S* }` is an expression: its value is the last statement. Arrow bodies may be blocks.

### 4.3 Conditionals and membership

```
if cond { a } else { b }
x in [1, 2, 3]       # list membership
"k" in {k: 1}        # map key
"bc" in "abcd"       # substring
```

`&&` and `||` short-circuit.

An infix/postfix operator on a **new line** does not continue the previous expression, except `| ?? && || + - * /`. So a `match` / lambda that ends with `}` is not indexed by a `[...]` on the next line.

## 5. Values and builtins

Types: `INTEGER FLOAT BOOLEAN STRING NULL LIST MAP FUNCTION BUILTIN`.

Equality is deep for lists and maps. Integer/float compare numerically.

Core builtins:

| Name | Role |
|---|---|
| `len keys values range append slice split join` | collections / strings |
| `ord chr is_digit is_alpha is_space` | character ops (for self-hosted lexing) |
| `int str float` | conversions |
| `copy apply` | map/list clone; `apply(fn, args_list)` |
| `must is_err` | abort on `{err:...}`; predicate |
| `??` | `x ?? 0` — use right if left is null or `{err:...}` |
| `builtins` | snapshot of the builtin map (seed a guest env) |
| `tool_call(name, args)` | invoke a registered tool by name |
| `json parse_json type print load use` | host I/O and modules |
| `vm_run vm_opcodes` | run a bytecode chunk `{code, consts, names}`; opcode name→int map |

`use "std/lexer"` evaluates the file in a **fresh** environment and returns an export map (the last map value, or all top-level bindings). After boot, `use`/`load` `vm_run` a sibling `.nlc` when it is newer than the source; otherwise they compile with `C.nl_parse`/`nl_compile` and write `.nlc`. Stale `std/*.nlc` are rebuilt by the guest compiler. Boot fails if the stage-0 `.nlc` files are missing. Paths resolve from CWD, the caller module directory, the repo root (`go.mod`), and the executable directory. `load("f.nl")` still injects bindings into the current env.

Tools (effects): `http.get` `http.post` `fs.list` `fs.read` `fs.write` `env.get` `time.now` `time.sleep`.

## 6. Self-hosting bootstrap

```
std/{lexer,parser,compile,compiler}.nlc  --vm_run-->  C.nl_eval
stale .nlc  --vm_run old chunks-->  guest nl_compile  --> write .nlc --> vm_run
```

`C.nl_eval(code, env)` = parse + compile + `vm_run`. `env=null` => `copy(builtins())`. `C.nl_compile(ast)` emits `{code, consts, names}` for `vm_run`. Parse errors from the self-hosted parser are `{type:"Err", msg, line, col}`. Bytecode closures are `{__bc, code, consts, names, params}`. `std/evaluator.nl` is a tree-walk over AST maps for debugging; it is not the eval path. The Go host is only the stack VM.

`neurolang run` / `eval` / `repl` boot from committed `std/{lexer,parser,compile,compiler}.nlc`. `compiler.nlc` is the full module (`use` lexer/parser/compile); that `use` `vm_run`s sibling `.nlc` before `C` is bound. If `.nlc` are older than the `.nl` sources, the guest compiler rewrites them. Missing stage-0 `.nlc` is a boot error. Language tests live in `tests/*.nl`; `neurolang check` runs them. Grow the language by desugaring in `std/` onto existing opcodes; touch the Go VM only for a new opcode, builtin, or value kind. After boot, `use`/`load` `vm_run` a sibling `.nlc` when it is newer than the source; otherwise they compile and write `.nlc`. `neurolang self` is an alias of `run`. `!ident` is a tool call; write boolean not as `!(expr)` or `x == false`.

CLI:

- `neurolang run file.nl` — boot `std/compiler`, then `C.nl_eval`
- `neurolang self file.nl` — alias of `run`
- `neurolang tools` — compact `!tool` catalog for models
- `neurolang mcp` — MCP stdio JSON-RPC (`initialize`, `tools/list`, `tools/call`)
- `neurolang check` — rebuild stale `std/*.nlc`, run `tests/*.nl`
- `neurolang spec` — prints `SPEC_AI.md` (the dense primer for models)

The self-hosted stack must run pipelines, functions, `if`/`for`/`while`, maps, assignment, and tools. That corpus lives in `tests/*.nl` and runs on `C.nl_eval`. Grow syntax in `std/` by desugaring to existing bytecode. The Go host is the VM, builtins, and tools.

## 7. Generation rules for agents

1. Prefer a pipeline over a `while` whenever the work is filter/map/reduce.
2. Keep identifiers short; the language is the compression.
3. Do not emit `import` or type annotations; bind tools with `!`.
4. When in doubt, follow `SPEC_AI.md` — it is the canonical compact grammar.
