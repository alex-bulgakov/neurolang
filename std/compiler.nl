# ==========================================================
# std/compiler.nl — self-hosted frontend: tokenize, parse, compile
# Boot vm_runs this module (and its use of lexer/parser/compile).
# std/evaluator.nl is a debug AST interpreter, not the eval path.
# ==========================================================

L = use "lexer.nl"
P = use "parser.nl"
K = use "compile.nl"

nl_eval = (code, env) -> {
  if env == null {
    env = copy(builtins())
  }
  ast = P.parse_program(L.tokenize(code))
  if len(ast.statements) > 0 && ast.statements[0].type == "Err" {
    e = ast.statements[0]
    return {err: e.msg, line: e.line, col: e.col}
  }
  vm_run(K.compile_program(ast), env)
}

nl_parse = code -> P.parse_program(L.tokenize(code))

nl_compile = ast -> K.compile_program(ast)

{nl_eval: nl_eval, nl_parse: nl_parse, nl_compile: nl_compile}
