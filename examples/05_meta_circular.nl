# ==========================================================
# 05_meta_circular.nl — self-hosted compiler proving ground
# ==========================================================

print("=== NeuroLang v0.8 self-hosted compiler ===")

C = use "std/compiler"
L = use "std/lexer"

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
ok = check("pipe", C.nl_eval("[10, 20, 30, 40] | ?(. > 15) | @(. * 2)", null), [40, 60, 80]) && ok
ok = check("lambda", C.nl_eval("square = n -> n * n\nsquare(8)", null), 64) && ok
ok = check("for", C.nl_eval("acc = 0\nfor n in [1, 2, 3] {\n  acc = acc + n\n}\nacc", null), 6) && ok
ok = check("if/map", C.nl_eval("u = {name: \"Ada\", active: true}\nif u.active { u.name } else { \"no\" }", null), "Ada") && ok
ok = check("in", C.nl_eval("3 in [1, 2, 3] && \"k\" in {k: 1}", null), true) && ok

guest = "xs | @(. + 1)"
toks = L.tokenize(guest)
ok = check("lex-count", len(toks), 9) && ok
ok = check("lex-line", toks[0].line, 1) && ok

bad = C.nl_parse(")\n")
ok = check("parse-err", bad.statements[0].type, "Err") && ok

if ok {
  print("")
  print(">>> SUCCESS: self-hosted compiler subset verified <<<")
} else {
  print("")
  print("FAILURE: one or more self-host checks failed")
}
