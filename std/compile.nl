# ==========================================================
# std/compile.nl — AST maps → bytecode for the host stack VM
# Opcodes come from vm_opcodes() so they stay aligned with Go.
# ==========================================================

OP = vm_opcodes()

new_c = () -> {code: [], consts: [], names: [], loops: [], hid: 0}

emit = (c, op) -> { c.code = append(c.code, op) }

emit1 = (c, op, a) -> {
  c.code = append(c.code, op)
  c.code = append(c.code, a / 256)
  c.code = append(c.code, a % 256)
}

emit2 = (c, op, a, b) -> {
  emit1(c, op, a)
  c.code = append(c.code, b / 256)
  c.code = append(c.code, b % 256)
}

emit_jump = (c, op) -> {
  pos = len(c.code)
  emit1(c, op, 0)
  pos
}

patch = (c, pos, target) -> {
  c.code[pos + 1] = target / 256
  c.code[pos + 2] = target % 256
}

add_const = (c, v) -> {
  c.consts = append(c.consts, v)
  len(c.consts) - 1
}

add_name = (c, n) -> {
  i = 0
  while i < len(c.names) {
    if c.names[i] == n { return i }
    i = i + 1
  }
  c.names = append(c.names, n)
  len(c.names) - 1
}

hidden = c -> {
  c.hid = c.hid + 1
  "__m" + str(c.hid)
}

finish = c -> {code: c.code, consts: c.consts, names: c.names}

binops = {
  "+": OP.add,
  "-": OP.sub,
  "*": OP.mul,
  "/": OP.div,
  "%": OP.mod,
  "==": OP.eq,
  "!=": OP.neq,
  "<": OP.lt,
  "<=": OP.lte,
  ">": OP.gt,
  ">=": OP.gte,
  "in": OP["in"]
}

compile_stmts = (stmts, c, keep) -> {
  if stmts == null || len(stmts) == 0 {
    if keep { emit(c, OP["null"]) }
    return null
  }
  i = 0
  while i < len(stmts) {
    k = keep && i == len(stmts) - 1
    compile_node(stmts[i], c, true)
    if k == false { emit(c, OP.pop) }
    i = i + 1
  }
}

compile_assign = (node, c) -> {
  compile_node(node.val, c, true)
  t = node.target
  if t == null {
    emit1(c, OP.set, add_name(c, node.name))
    return null
  }
  if t.type == "Ident" {
    emit1(c, OP.set, add_name(c, t.name))
    return null
  }
  if t.type == "Prop" {
    compile_node(t.left, c, true)
    emit1(c, OP.set_prop, add_name(c, t.field))
    return null
  }
  if t.type == "Index" {
    compile_node(t.left, c, true)
    compile_node(t.index, c, true)
    emit(c, OP.set_index)
    return null
  }
  emit(c, OP.pop)
  emit(c, OP["null"])
}

compile_if = (node, c) -> {
  compile_node(node.cond, c, true)
  jfalse = emit_jump(c, OP.jump_false)
  emit(c, OP.pop)
  compile_node(node.then, c, true)
  jend = emit_jump(c, OP.jump)
  patch(c, jfalse, len(c.code))
  emit(c, OP.pop)
  if node.alt != null {
    compile_node(node.alt, c, true)
  } else {
    emit(c, OP["null"])
  }
  patch(c, jend, len(c.code))
}

compile_while = (node, c) -> {
  start = len(c.code)
  c.loops = append(c.loops, {start: start, breaks: [], continues: [], pops: 0})
  compile_node(node.cond, c, true)
  jfalse = emit_jump(c, OP.jump_false)
  emit(c, OP.pop)
  compile_node(node.body, c, false)
  emit1(c, OP.jump, start)
  fail = len(c.code)
  patch(c, jfalse, fail)
  emit(c, OP.pop)
  end = len(c.code)
  emit(c, OP["null"])
  lp = c.loops[len(c.loops) - 1]
  c.loops = slice(c.loops, 0, len(c.loops) - 1)
  for b in lp.breaks { patch(c, b, end) }
  for ct in lp.continues { patch(c, ct, start) }
}

compile_for = (node, c) -> {
  compile_node(node.iter, c, true)
  emit(c, OP.iter)
  start = len(c.code)
  c.loops = append(c.loops, {start: start, breaks: [], continues: [], pops: 1})
  jdone = emit_jump(c, OP.iter_next)
  emit(c, OP.set_dot)
  emit1(c, OP.set, add_name(c, node.name))
  emit(c, OP.pop)
  compile_node(node.body, c, false)
  emit1(c, OP.jump, start)
  end = len(c.code)
  patch(c, jdone, end)
  emit(c, OP["null"])
  lp = c.loops[len(c.loops) - 1]
  c.loops = slice(c.loops, 0, len(c.loops) - 1)
  for b in lp.breaks { patch(c, b, end) }
  for ct in lp.continues { patch(c, ct, start) }
}

compile_match = (node, c) -> {
  hid = hidden(c)
  compile_node(node.subj, c, true)
  emit1(c, OP.set, add_name(c, hid))
  emit(c, OP.pop)
  ends = []
  for cs in node.cases {
    if cs.pat.type == "Ident" && cs.pat.name == "_" {
      compile_node(cs.body, c, true)
      ends = append(ends, emit_jump(c, OP.jump))
    } else {
      compile_node(cs.pat, c, true)
      emit1(c, OP.get, add_name(c, hid))
      emit(c, OP.eq)
      jfalse = emit_jump(c, OP.jump_false)
      emit(c, OP.pop)
      compile_node(cs.body, c, true)
      ends = append(ends, emit_jump(c, OP.jump))
      patch(c, jfalse, len(c.code))
      emit(c, OP.pop)
    }
  }
  emit(c, OP["null"])
  end = len(c.code)
  for e in ends { patch(c, e, end) }
}

compile_lambda = (node, c) -> {
  inner = new_c()
  compile_node(node.body, inner, true)
  emit(inner, OP["return"])
  idx = add_const(c, {__bc: true, code: inner.code, consts: inner.consts, names: inner.names, params: node.params})
  emit1(c, OP.closure, idx)
}

compile_pipe_rhs = (right, c) -> {
  rt = right.type
  if rt == "Filter" {
    inner = new_c()
    compile_node(right.pred, inner, true)
    emit(inner, OP["return"])
    emit1(c, OP.closure, add_const(c, {__bc: true, code: inner.code, consts: inner.consts, names: inner.names, params: []}))
    emit(c, OP.pipe_filter)
    return null
  }
  if rt == "MapOp" {
    inner = new_c()
    compile_node(right.transform, inner, true)
    emit(inner, OP["return"])
    emit1(c, OP.closure, add_const(c, {__bc: true, code: inner.code, consts: inner.consts, names: inner.names, params: []}))
    emit(c, OP.pipe_map)
    return null
  }
  if rt == "Reduce" {
    compile_node(right.reducer, c, true)
    emit(c, OP.pipe_reduce)
    return null
  }
  if rt == "ToolCall" {
    for a in right.args { compile_node(a, c, true) }
    emit2(c, OP.tool, add_name(c, right.name), len(right.args) + 1)
    return null
  }
  if rt == "Call" {
    compile_node(right.callee, c, true)
    for a in right.args { compile_node(a, c, true) }
    emit1(c, OP.pipe_call, len(right.args) + 1)
    return null
  }
  compile_node(right, c, true)
  emit1(c, OP.pipe_call, 1)
}

compile_infix = (node, c) -> {
  op = node.op
  if op == "&&" {
    compile_node(node.left, c, true)
    jfalse = emit_jump(c, OP.jump_false)
    emit(c, OP.pop)
    compile_node(node.right, c, true)
    emit(c, OP.to_bool)
    jend = emit_jump(c, OP.jump)
    patch(c, jfalse, len(c.code))
    emit(c, OP.pop)
    emit(c, OP["false"])
    patch(c, jend, len(c.code))
    return null
  }
  if op == "||" {
    compile_node(node.left, c, true)
    jtrue = emit_jump(c, OP.jump_true)
    emit(c, OP.pop)
    compile_node(node.right, c, true)
    emit(c, OP.to_bool)
    jend = emit_jump(c, OP.jump)
    patch(c, jtrue, len(c.code))
    emit(c, OP.pop)
    emit(c, OP["true"])
    patch(c, jend, len(c.code))
    return null
  }
  if op == "??" {
    compile_node(node.left, c, true)
    jok = emit_jump(c, OP.jump_ok)
    emit(c, OP.pop)
    compile_node(node.right, c, true)
    patch(c, jok, len(c.code))
    return null
  }
  compile_node(node.left, c, true)
  compile_node(node.right, c, true)
  emit(c, binops[op])
}

compile_node = (node, c, keep) -> {
  if node == null {
    emit(c, OP["null"])
    return null
  }
  nt = node.type
  if nt == "Program" || nt == "Block" {
    compile_stmts(node.statements, c, keep)
    return null
  }
  if nt == "Assign" { compile_assign(node, c); return null }
  if nt == "Return" {
    compile_node(node.val, c, true)
    emit(c, OP["return"])
    return null
  }
  if nt == "While" { compile_while(node, c); return null }
  if nt == "For" { compile_for(node, c); return null }
  if nt == "Break" {
    lp = c.loops[len(c.loops) - 1]
    i = 0
    while i < lp.pops {
      emit(c, OP.pop)
      i = i + 1
    }
    j = emit_jump(c, OP.jump)
    lp.breaks = append(lp.breaks, j)
    return null
  }
  if nt == "Continue" {
    lp = c.loops[len(c.loops) - 1]
    j = emit_jump(c, OP.jump)
    lp.continues = append(lp.continues, j)
    return null
  }
  if nt == "Int" { emit1(c, OP.constant, add_const(c, int(node.val))); return null }
  if nt == "Float" { emit1(c, OP.constant, add_const(c, float(node.val))); return null }
  if nt == "String" { emit1(c, OP.constant, add_const(c, node.val)); return null }
  if nt == "Bool" {
    if node.val { emit(c, OP["true"]) } else { emit(c, OP["false"]) }
    return null
  }
  if nt == "Null" { emit(c, OP["null"]); return null }
  if nt == "Ident" { emit1(c, OP.get, add_name(c, node.name)); return null }
  if nt == "Prefix" {
    compile_node(node.right, c, true)
    if node.op == "-" { emit(c, OP.minus) } else { emit(c, OP.bang) }
    return null
  }
  if nt == "BinOp" { compile_infix(node, c); return null }
  if nt == "List" {
    for el in node.elements { compile_node(el, c, true) }
    emit1(c, OP.array, len(node.elements))
    return null
  }
  if nt == "Map" {
    for p in node.pairs {
      emit1(c, OP.constant, add_const(c, p.k))
      compile_node(p.v, c, true)
    }
    emit1(c, OP.map, len(node.pairs))
    return null
  }
  if nt == "Index" {
    compile_node(node.left, c, true)
    compile_node(node.index, c, true)
    emit(c, OP.index)
    return null
  }
  if nt == "Prop" {
    compile_node(node.left, c, true)
    emit1(c, OP.prop, add_name(c, node.field))
    return null
  }
  if nt == "Dot" {
    if node.field == "" { emit(c, OP.dot) } else { emit1(c, OP.dot_field, add_name(c, node.field)) }
    return null
  }
  if nt == "Call" {
    compile_node(node.callee, c, true)
    for a in node.args { compile_node(a, c, true) }
    emit1(c, OP.call, len(node.args))
    return null
  }
  if nt == "Lambda" { compile_lambda(node, c); return null }
  if nt == "If" { compile_if(node, c); return null }
  if nt == "Match" { compile_match(node, c); return null }
  if nt == "Use" {
    compile_node(node.path, c, true)
    emit(c, OP["use"])
    return null
  }
  if nt == "ToolCall" {
    for a in node.args { compile_node(a, c, true) }
    emit2(c, OP.tool, add_name(c, node.name), len(node.args))
    return null
  }
  if nt == "Pipe" {
    compile_node(node.left, c, true)
    compile_pipe_rhs(node.right, c)
    return null
  }
  if nt == "Filter" || nt == "MapOp" || nt == "Reduce" {
    emit(c, OP.dot)
    compile_pipe_rhs(node, c)
    return null
  }
  if nt == "Err" {
    emit1(c, OP.constant, add_const(c, {err: node.msg, line: node.line, col: node.col}))
    return null
  }
  emit(c, OP["null"])
}

compile_program = node -> {
  c = new_c()
  compile_node(node, c, true)
  finish(c)
}

{compile_program: compile_program}
