# ==========================================================
# 05_meta_circular.nl — self-hosted compiler proving ground
# Host (Go) loads std/*.nl, then that stack evaluates guest NL.
# ==========================================================

print("=== NeuroLang v0.3 self-hosted compiler ===")

load("std/lexer.nl")
load("std/parser.nl")
load("std/evaluator.nl")
load("std/compiler.nl")

check = (name, got, want) -> {
  if got == want {
    print("  ok  ", name, "=>", got)
    true
  } else {
    print("  FAIL", name, "got", got, "want", want)
    false
  }
}

ok = true

# 1. Dataflow combinators
ok = check("pipe", nl_eval("[10, 20, 30, 40] | ?(. > 15) | @(. * 2)", null), [40, 60, 80]) && ok

# 2. Functions + calls
ok = check("lambda", nl_eval("square = n -> n * n\nsquare(8)", null), 64) && ok

# 3. for-in + assignment
ok = check("for", nl_eval("acc = 0\nfor n in [1, 2, 3] {\n  acc = acc + n\n}\nacc", null), 6) && ok

# 4. maps, property, if
ok = check("if/map", nl_eval("u = {name: \"Ada\", active: true}\nif u.active { u.name } else { \"no\" }", null), "Ada") && ok

# 5. membership
ok = check("in", nl_eval("3 in [1, 2, 3] && \"k\" in {k: 1}", null), true) && ok

# 6. Self-hosted lexer tokenizes a guest snippet (compiler eating compiler input)
guest = "xs | @(. + 1)"
toks = tokenize(guest)
ok = check("lex-count", len(toks), 9) && ok

if ok {
  print("")
  print(">>> SUCCESS: self-hosted compiler subset verified <<<")
} else {
  print("")
  print("FAILURE: one or more self-host checks failed")
}
