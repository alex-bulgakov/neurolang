# ==========================================================
# std/compiler.nl — self-hosted frontend: tokenize, parse, eval
# ==========================================================

L = use "lexer.nl"
P = use "parser.nl"
E = use "evaluator.nl"

nl_eval = (code, env) -> {
  if env == null {
    env = copy(builtins())
  }
  ast = P.parse_program(L.tokenize(code))
  E.unwrap(E.eval_ast(ast, env, null))
}

nl_parse = code -> P.parse_program(L.tokenize(code))

{nl_eval: nl_eval, nl_parse: nl_parse}
