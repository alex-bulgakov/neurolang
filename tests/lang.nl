# Language corpus. Mutate acc.bad (map is shared); do not use must() here.
acc = {bad: []}
eq = (got, want, name) -> {
  if got != want { acc.bad = append(acc.bad, name) }
  true
}

eq(5 + 2 * 10, 25, "arith")
eq(-50 + 100 + -50, 0, "arith2")
eq((5 + 10 * 2 + 15 / 3) * 2 + -10, 50, "arith3")
eq([1, 2, 3, 4, 5] | ?(. > 3), [4, 5], "filter")
eq([1, 2, 3] | @(. * 10), [10, 20, 30], "map")
eq([1, 2, 3, 4] | &((a, b) -> a + b), 10, "reduce")

users = [
  {name: "Alice", age: 25, active: true},
  {name: "Bob", age: 17, active: true},
  {name: "Charlie", age: 30, active: false}
]
eq(users | ?(.active && .age >= 18) | @.name, ["Alice"], "users")

makeAdder = x -> (y -> x + y)
eq(makeAdder(2)(3), 5, "closure")

classify = n -> match n {
  0 -> "zero"
  1 -> "one"
  _ -> "many"
}
eq([classify(0), classify(1), classify(99)], ["zero", "one", "many"], "match")

i = 0
total = 0
while i < 10 {
  i = i + 1
  if i == 5 { continue }
  if i > 8 { break }
  total = total + i
}
eq(total, 31, "while")

accn = 0
for n in [1, 2, 3, 4] { accn = accn + n }
eq(accn, 10, "for")
eq(3 in [1, 2, 3], true, "in-list")
eq("name" in {name: "Ada"}, true, "in-map")
eq("bc" in "abcd", true, "in-str")
eq(null ?? 7, 7, "coalesce")
eq({err: "boom"} ?? 9, 9, "coalesce-err")
eq(must(4), 4, "must-ok")
eq(false && (1 / 0), false, "and")
eq(true || (1 / 0), true, "or")

obj = {count: 1}
obj.count = obj.count + 4
obj["tag"] = "ok"
eq(obj.count + len(obj), 7, "assign")

a = [1, 2]
eq(slice(a + [3, 4], 1, 3), [2, 3], "slice")
eq([chr(65), ord("B"), is_digit("7"), is_alpha("_")], ["A", 66, true, true], "chars")

m = {a: 1}
c = copy(m)
c.a = 2
eq(m.a, 1, "copy")
eq(apply(x -> x + 1, [3]), 4, "apply")

L = use "std/lexer"
eq(L.tokenize("a = 1")[0].type, "IDENT", "use-lexer")

if len(acc.bad) > 0 { must({err: join(acc.bad, ",")}) }
"ok"
