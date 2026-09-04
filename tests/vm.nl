# Guest VM (std/vm.nl) and C emit helper.
acc = {bad: []}
eq = (got, want, name) -> {
  if got != want { acc.bad = append(acc.bad, name) }
  true
}

C = use "std/compiler"
V = use "std/vm"
E = use "std/emit_c"

run = src -> V.run(C.nl_compile(C.nl_parse(src)), copy(builtins()))
eq(run("5 + 2 * 10"), 25, "vm-arith")
eq(run("x = 40\nx + 2"), 42, "vm-var")
eq(run("add = (a, b) -> a + b\nadd(3, 4)"), 7, "vm-fn")
eq(run("[1, 2, 3]"), [1, 2, 3], "vm-list")

bc = C.nl_compile(C.nl_parse("1+2"))
eq(E.emit_ints(bc.code) != "", true, "emit-ints")

if len(acc.bad) > 0 { must({err: join(acc.bad, ",")}) }
"ok"
