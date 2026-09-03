# ==========================================================
# std/parser.nl — Pratt parser for the NeuroLang compiler subset
# ==========================================================

precedences = {
  "|": 10,
  "||": 20,
  "&&": 30,
  "in": 50,
  "==": 40,
  "!=": 40,
  "<": 50,
  "<=": 50,
  ">": 50,
  ">=": 50,
  "+": 60,
  "-": 60,
  "*": 70,
  "/": 70,
  "%": 70,
  "(": 80,
  "[": 80,
  ".": 80
}

cur = state -> {
  if state.pos >= len(state.tokens) {
    {type: "EOF", literal: ""}
  } else {
    state.tokens[state.pos]
  }
}

look = (state, n) -> {
  i = state.pos + n
  if i >= len(state.tokens) {
    {type: "EOF", literal: ""}
  } else {
    state.tokens[i]
  }
}

adv = state -> {
  t = cur(state)
  state.pos = state.pos + 1
  t
}

skip_if = (state, typ) -> {
  if cur(state).type == typ {
    adv(state)
    true
  } else {
    false
  }
}

get_prec = tok_type -> {
  p = precedences[tok_type]
  if p == null { 0 } else { p }
}

parse_pattern = state -> {
  t = cur(state)
  if t.type == "IDENT" { adv(state); return {type: "Ident", name: t.literal} }
  if t.type == "INT" { adv(state); return {type: "Int", val: t.literal} }
  if t.type == "FLOAT" { adv(state); return {type: "Float", val: t.literal} }
  if t.type == "STRING" { adv(state); return {type: "String", val: t.literal} }
  if t.type == "true" { adv(state); return {type: "Bool", val: true} }
  if t.type == "false" { adv(state); return {type: "Bool", val: false} }
  if t.type == "null" { adv(state); return {type: "Null"} }
  parse_expr(state, 90)
}

make_err = (state, msg) -> {
  t = cur(state)
  ln = t.line
  cl = t.col
  if ln == null { ln = 0 }
  if cl == null { cl = 0 }
  {type: "Err", msg: msg, line: ln, col: cl}
}

parse_primary = state -> {
  t = cur(state)

  if t.type == "INT" { adv(state); return {type: "Int", val: t.literal} }
  if t.type == "FLOAT" { adv(state); return {type: "Float", val: t.literal} }
  if t.type == "STRING" { adv(state); return {type: "String", val: t.literal} }
  if t.type == "true" { adv(state); return {type: "Bool", val: true} }
  if t.type == "false" { adv(state); return {type: "Bool", val: false} }
  if t.type == "null" { adv(state); return {type: "Null"} }

  if t.type == "IDENT" {
    adv(state)
    if cur(state).type == "->" {
      adv(state)
      return {type: "Lambda", params: [t.literal], body: parse_expr(state, 0)}
    }
    return {type: "Ident", name: t.literal}
  }

  if t.type == "." {
    adv(state)
    field = ""
    if cur(state).type == "IDENT" {
      field = adv(state).literal
      while cur(state).type == "." && look(state, 1).type == "IDENT" {
        adv(state)
        field = field + "." + adv(state).literal
      }
    }
    return {type: "Dot", field: field}
  }

  if t.type == "[" {
    adv(state)
    elements = []
    while cur(state).type != "]" && cur(state).type != "EOF" {
      elements = append(elements, parse_expr(state, 0))
      skip_if(state, ",")
    }
    adv(state)
    return {type: "List", elements: elements}
  }

  if t.type == "{" {
    adv(state)
    if cur(state).type == "}" {
      adv(state)
      return {type: "Map", pairs: []}
    }
    t0 = cur(state)
    t1 = look(state, 1)
    if (t0.type == "IDENT" || t0.type == "STRING") && t1.type == ":" {
      pairs = []
      while cur(state).type != "}" && cur(state).type != "EOF" {
        kt = adv(state)
        adv(state)
        pairs = append(pairs, {k: kt.literal, v: parse_expr(state, 0)})
        skip_if(state, ",")
      }
      adv(state)
      return {type: "Map", pairs: pairs}
    }
    stmts = []
    while cur(state).type != "}" && cur(state).type != "EOF" {
      s = parse_statement(state)
      if s != null {
        stmts = append(stmts, s)
      }
    }
    adv(state)
    return {type: "Block", statements: stmts}
  }

  if t.type == "(" {
    adv(state)
    if cur(state).type == ")" {
      adv(state)
      if cur(state).type == "->" {
        adv(state)
        return {type: "Lambda", params: [], body: parse_expr(state, 0)}
      }
      return {type: "Null"}
    }
    first = parse_expr(state, 0)
    if cur(state).type == "," {
      params = []
      if first.type == "Ident" {
        params = append(params, first.name)
      }
      while cur(state).type == "," {
        adv(state)
        params = append(params, adv(state).literal)
      }
      if cur(state).type == ")" { adv(state) }
      if cur(state).type == "->" { adv(state) }
      return {type: "Lambda", params: params, body: parse_expr(state, 0)}
    }
    if cur(state).type == ")" { adv(state) }
    if cur(state).type == "->" {
      adv(state)
      params = []
      if first.type == "Ident" {
        params = [first.name]
      }
      return {type: "Lambda", params: params, body: parse_expr(state, 0)}
    }
    return first
  }

  if t.type == "?" { adv(state); return {type: "Filter", pred: parse_expr(state, 75)} }
  if t.type == "@" { adv(state); return {type: "MapOp", transform: parse_expr(state, 75)} }
  if t.type == "&" { adv(state); return {type: "Reduce", reducer: parse_expr(state, 75)} }

  if t.type == "!" {
    if look(state, 1).type == "IDENT" {
      adv(state)
      name = adv(state).literal
      while cur(state).type == "." {
        adv(state)
        name = name + "." + adv(state).literal
      }
      args = []
      if cur(state).type == "(" {
        adv(state)
        while cur(state).type != ")" && cur(state).type != "EOF" {
          args = append(args, parse_expr(state, 0))
          skip_if(state, ",")
        }
        adv(state)
      }
      return {type: "ToolCall", name: name, args: args}
    }
    adv(state)
    return {type: "Prefix", op: "!", right: parse_expr(state, 75)}
  }

  if t.type == "-" {
    adv(state)
    return {type: "Prefix", op: "-", right: parse_expr(state, 75)}
  }

  if t.type == "if" {
    adv(state)
    cond = parse_expr(state, 0)
    then_b = parse_primary(state)
    alt = null
    if cur(state).type == "else" {
      adv(state)
      alt = parse_primary(state)
    }
    return {type: "If", cond: cond, then: then_b, alt: alt}
  }

  if t.type == "match" {
    adv(state)
    subj = parse_expr(state, 0)
    if cur(state).type == "{" { adv(state) }
    cases = []
    while cur(state).type != "}" && cur(state).type != "EOF" {
      pat = parse_pattern(state)
      if cur(state).type == "->" { adv(state) }
      cases = append(cases, {pat: pat, body: parse_expr(state, 0)})
      skip_if(state, ",")
      skip_if(state, ";")
    }
    adv(state)
    return {type: "Match", subj: subj, cases: cases}
  }

  if t.type == "use" {
    adv(state)
    return {type: "Use", path: parse_expr(state, 75)}
  }

  adv(state)
  {type: "Err", msg: "no prefix for " + str(t.type), line: t.line, col: t.col}
}

parse_expr = (state, prec) -> {
  left = parse_primary(state)
  if left == null || left.type == "Err" {
    return left
  }

  while cur(state).type != "EOF" && prec < get_prec(cur(state).type) {
    op = cur(state).type
    adv(state)

    if op == "(" {
      args = []
      while cur(state).type != ")" && cur(state).type != "EOF" {
        args = append(args, parse_expr(state, 0))
        skip_if(state, ",")
      }
      adv(state)
      left = {type: "Call", callee: left, args: args}
    } else if op == "[" {
      idx = parse_expr(state, 0)
      if cur(state).type == "]" { adv(state) }
      left = {type: "Index", left: left, index: idx}
    } else if op == "." {
      field = ""
      if cur(state).type == "IDENT" {
        field = adv(state).literal
      }
      left = {type: "Prop", left: left, field: field}
    } else if op == "|" {
      left = {type: "Pipe", left: left, right: parse_expr(state, get_prec(op))}
    } else {
      left = {type: "BinOp", op: op, left: left, right: parse_expr(state, get_prec(op))}
    }
  }

  left
}

parse_statement = state -> {
  t = cur(state)

  if t.type == "return" {
    adv(state)
    if cur(state).type == "}" || cur(state).type == "EOF" || cur(state).type == ";" {
      skip_if(state, ";")
      return {type: "Return", val: {type: "Null"}}
    }
    val = parse_expr(state, 0)
    skip_if(state, ";")
    return {type: "Return", val: val}
  }

  if t.type == "while" {
    adv(state)
    cond = parse_expr(state, 0)
    return {type: "While", cond: cond, body: parse_primary(state)}
  }

  if t.type == "for" {
    adv(state)
    name = adv(state).literal
    if cur(state).type == "in" { adv(state) }
    iter = parse_expr(state, 0)
    return {type: "For", name: name, iter: iter, body: parse_primary(state)}
  }

  if t.type == "break" {
    adv(state)
    skip_if(state, ";")
    return {type: "Break"}
  }

  if t.type == "continue" {
    adv(state)
    skip_if(state, ";")
    return {type: "Continue"}
  }

  expr = parse_expr(state, 0)
  if expr == null || expr.type == "Err" {
    if expr != null && expr.type == "Err" {
      return expr
    }
    adv(state)
    return null
  }

  if cur(state).type == "=" {
    adv(state)
    val = parse_expr(state, 0)
    skip_if(state, ";")
    return {type: "Assign", target: expr, val: val}
  }

  skip_if(state, ";")
  expr
}

parse_program = tokens -> {
  state = {tokens: tokens, pos: 0}
  stmts = []
  while cur(state).type != "EOF" {
    s = parse_statement(state)
    if s != null {
      stmts = append(stmts, s)
    }
  }
  {type: "Program", statements: stmts}
}

{parse_program: parse_program}
