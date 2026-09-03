# ==========================================================
# std/compiler.nl — host bootstrap for the self-hosted compiler
# Requires std/lexer.nl, std/parser.nl, std/evaluator.nl
# ==========================================================

nl_eval = (code, env) -> {
  if env == null {
    env = copy(builtins())
  }
  unwrap(eval_ast(parse_program(tokenize(code)), env, null))
}

nl_parse = code -> parse_program(tokenize(code))
