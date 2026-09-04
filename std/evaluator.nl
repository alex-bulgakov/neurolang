# ==========================================================
# std/evaluator.nl — debug tree-walk over AST maps. Not used by
# neurolang run / eval / repl (those are C.nl_eval → vm_run).
# Closures are maps {__fn: true, params, body, env, dot}.
# Control-flow signals: {__sig: "return"|"break"|"continue", val}
# ==========================================================

is_sig = (v, kind) -> type(v) == "MAP" && v.__sig == kind

unwrap = v -> {
  if is_sig(v, "return") { v.val } else { v }
}

call_fn = (f, args) -> {
  if f == null {
    return null
  }
  if type(f) == "MAP" && f.__fn {
    new_env = copy(f.env)
    i = 0
    while i < len(f.params) {
      if i < len(args) {
        new_env[f.params[i]] = args[i]
      }
      i = i + 1
    }
    return unwrap(eval_ast(f.body, new_env, f.dot))
  }
  apply(f, args)
}

eval_ast = (node, env, dot) -> {
  if node == null {
    return null
  }

  nt = node.type

  if nt == "Int" { return int(node.val) }
  if nt == "Float" { return float(node.val) }
  if nt == "String" { return node.val }
  if nt == "Bool" { return node.val }
  if nt == "Null" { return null }

  if nt == "Err" {
    return {err: node.msg, line: node.line, col: node.col}
  }

  if nt == "Use" {
    return use(eval_ast(node.path, env, dot))
  }

  if nt == "Ident" { return env[node.name] }

  if nt == "Dot" {
    if node.field == "" { return dot }
    cur_v = dot
    for part in split(node.field, ".") {
      cur_v = cur_v[part]
    }
    return cur_v
  }

  if nt == "List" {
    return node.elements | @(eval_ast(., env, dot))
  }

  if nt == "Map" {
    m = {}
    for p in node.pairs {
      m[p.k] = eval_ast(p.v, env, dot)
    }
    return m
  }

  if nt == "Block" || nt == "Program" {
    last = null
    for s in node.statements {
      last = eval_ast(s, env, dot)
      if type(last) == "MAP" && last.err != null {
        return last
      }
      if type(last) == "MAP" && last.__sig != null {
        return last
      }
    }
    return last
  }

  if nt == "Assign" {
    val = eval_ast(node.val, env, dot)
    t = node.target
    if t == null {
      env[node.name] = val
      return val
    }
    if t.type == "Ident" {
      env[t.name] = val
      return val
    }
    if t.type == "Prop" {
      obj = eval_ast(t.left, env, dot)
      obj[t.field] = val
      return val
    }
    if t.type == "Index" {
      obj = eval_ast(t.left, env, dot)
      idx = eval_ast(t.index, env, dot)
      obj[idx] = val
      return val
    }
    return val
  }

  if nt == "Prefix" {
    r = eval_ast(node.right, env, dot)
    if node.op == "-" { return -r }
    if node.op == "!" {
      if r { return false } else { return true }
    }
    return r
  }

  if nt == "BinOp" {
    op = node.op
    if op == "&&" {
      l = eval_ast(node.left, env, dot)
      if l {
        r = eval_ast(node.right, env, dot)
        if r { return true } else { return false }
      }
      return false
    }
    if op == "||" {
      l = eval_ast(node.left, env, dot)
      if l { return true }
      r = eval_ast(node.right, env, dot)
      if r { return true } else { return false }
    }
    if op == "??" {
      l = eval_ast(node.left, env, dot)
      if l == null || (type(l) == "MAP" && l.err != null) {
        return eval_ast(node.right, env, dot)
      }
      return l
    }
    l = eval_ast(node.left, env, dot)
    r = eval_ast(node.right, env, dot)
    if op == "+" { return l + r }
    if op == "-" { return l - r }
    if op == "*" { return l * r }
    if op == "/" { return l / r }
    if op == "%" { return l % r }
    if op == "==" { return l == r }
    if op == "!=" { return l != r }
    if op == "<" { return l < r }
    if op == "<=" { return l <= r }
    if op == ">" { return l > r }
    if op == ">=" { return l >= r }
    if op == "in" { return l in r }
    return null
  }

  if nt == "Lambda" {
    return {__fn: true, params: node.params, body: node.body, env: env, dot: dot}
  }

  if nt == "Call" {
    f = eval_ast(node.callee, env, dot)
    args = node.args | @(eval_ast(., env, dot))
    return call_fn(f, args)
  }

  if nt == "Prop" {
    obj = eval_ast(node.left, env, dot)
    return obj[node.field]
  }

  if nt == "Index" {
    obj = eval_ast(node.left, env, dot)
    idx = eval_ast(node.index, env, dot)
    return obj[idx]
  }

  if nt == "If" {
    if eval_ast(node.cond, env, dot) {
      return eval_ast(node.then, env, dot)
    }
    if node.alt != null {
      return eval_ast(node.alt, env, dot)
    }
    return null
  }

  if nt == "While" {
    while eval_ast(node.cond, env, dot) {
      res = eval_ast(node.body, env, dot)
      if is_sig(res, "break") { break }
      if is_sig(res, "continue") { continue }
      if is_sig(res, "return") { return res }
    }
    return null
  }

  if nt == "For" {
    xs = eval_ast(node.iter, env, dot)
    for x in xs {
      env[node.name] = x
      res = eval_ast(node.body, env, x)
      if is_sig(res, "break") { break }
      if is_sig(res, "continue") { continue }
      if is_sig(res, "return") { return res }
    }
    return null
  }

  if nt == "Return" { return {__sig: "return", val: eval_ast(node.val, env, dot)} }
  if nt == "Break" { return {__sig: "break"} }
  if nt == "Continue" { return {__sig: "continue"} }

  if nt == "Match" {
    subj = eval_ast(node.subj, env, dot)
    for c in node.cases {
      if c.pat.type == "Ident" && c.pat.name == "_" {
        return eval_ast(c.body, env, dot)
      }
      if eval_ast(c.pat, env, dot) == subj {
        return eval_ast(c.body, env, dot)
      }
    }
    return null
  }

  if nt == "Pipe" {
    left_val = eval_ast(node.left, env, dot)
    rt = node.right.type

    if rt == "Filter" {
      return left_val | ?(eval_ast(node.right.pred, env, .))
    }
    if rt == "MapOp" {
      return left_val | @(eval_ast(node.right.transform, env, .))
    }
    if rt == "Reduce" {
      f = eval_ast(node.right.reducer, env, dot)
      if len(left_val) == 0 { return null }
      acc = left_val[0]
      i = 1
      while i < len(left_val) {
        acc = call_fn(f, [acc, left_val[i]])
        i = i + 1
      }
      return acc
    }
    if rt == "ToolCall" {
      args = [left_val] + (node.right.args | @(eval_ast(., env, dot)))
      return tool_call(node.right.name, args)
    }
    if rt == "Call" {
      f = eval_ast(node.right.callee, env, dot)
      args = [left_val] + (node.right.args | @(eval_ast(., env, dot)))
      return call_fn(f, args)
    }

    f = eval_ast(node.right, env, dot)
    return call_fn(f, [left_val])
  }

  if nt == "Filter" {
    return eval_ast(node.pred, env, dot)
  }
  if nt == "MapOp" {
    return eval_ast(node.transform, env, dot)
  }

  if nt == "ToolCall" {
    args = node.args | @(eval_ast(., env, dot))
    return tool_call(node.name, args)
  }

  null
}

{eval_ast: eval_ast, unwrap: unwrap}
