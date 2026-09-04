# Guest compiler: nl_eval / parse errors / dogfood std/*.nl
acc = {bad: []}
eq = (got, want, name) -> {
  if got != want { acc.bad = append(acc.bad, name) }
  true
}

C = use "std/compiler"
eq(C.nl_eval("5 + 2 * 10", null), 25, "nl_eval")
eq(C.nl_eval("square = n -> n * n\nsquare(8)", null), 64, "nl_eval-fn")

n = C.nl_parse(")\n").statements[0]
eq([n.type, n.line, n.col], ["Err", 1, 1], "parse-err")

bc = C.nl_compile(C.nl_parse("acc = 0\nfor n in [1, 2, 3] {\n  acc = acc + n\n}\nacc"))
eq(vm_run(bc, copy(builtins())), 6, "compile-run")

compile_file = path -> {
  ast = C.nl_parse(!fs.read(path))
  if len(ast.statements) == 0 { return {err: "empty"} }
  if ast.statements[0].type == "Err" { return ast.statements[0] }
  C.nl_compile(ast)
}
chunk_ok = bc -> type(bc) == "MAP" && len(bc.code) > 0
lex_bc = compile_file("std/lexer.nl")
par_bc = compile_file("std/parser.nl")
cmp_bc = compile_file("std/compile.nl")
front_bc = compile_file("std/compiler.nl")
eq([chunk_ok(lex_bc), chunk_ok(par_bc), chunk_ok(cmp_bc), chunk_ok(front_bc)], [true, true, true, true], "dogfood-chunks")

L = vm_run(lex_bc, copy(builtins()))
P = vm_run(par_bc, copy(builtins()))
K = vm_run(cmp_bc, copy(builtins()))
out = vm_run(K.compile_program(P.parse_program(L.tokenize("acc = 1 + 2"))), copy(builtins()))
eq(out, 3, "dogfood-run")

if len(acc.bad) > 0 { must({err: join(acc.bad, ",")}) }
"ok"
