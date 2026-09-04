# ==========================================================
# std/vm.nl — bytecode VM in NeuroLang. Source of truth for the
# runtime. Stage-0 native host is rt/*.c until emit_c can AOT this.
# ==========================================================

OP = vm_opcodes()

u16 = (code, ip) -> code[ip] * 256 + code[ip + 1]

push = (st, v) -> {
  if st.n == len(st.items) { st.items = append(st.items, v) } else { st.items[st.n] = v }
  st.n = st.n + 1
}

pop = st -> {
  st.n = st.n - 1
  st.items[st.n]
}

peek = st -> st.items[st.n - 1]

env_get = (e, k) -> {
  while e != null {
    if k in e.store { return e.store[k] }
    e = e.outer
  }
  null
}

env_set = (e, k, v) -> { e.store[k] = v }

as_fn = v -> {
  if type(v) != "MAP" { return v }
  if v.__fn { return v }
  if v.__bc || v.code != null {
    return {__fn: true, code: v.code, consts: v.consts, names: v.names, params: v.params ?? [], env: null}
  }
  v
}

truthy = v -> {
  if v == null || v == false { return false }
  if v == true { return true }
  t = type(v)
  if t == "INTEGER" { return v != 0 }
  if t == "FLOAT" { return v != 0.0 }
  if t == "STRING" { return len(v) > 0 }
  if t == "LIST" { return len(v) > 0 }
  true
}

run = (ch, env) -> {
  if env == null { env = {store: copy(builtins()), outer: null, dot: null} }
  if env.store == null { env = {store: env, outer: null, dot: null} }
  code = ch.code
  consts = ch.consts
  names = ch.names
  st = {items: [], n: 0}
  ip = 0
  while ip < len(code) {
    op = code[ip]
    ip = ip + 1
    if op == OP.constant {
      idx = u16(code, ip)
      ip = ip + 2
      push(st, as_fn(consts[idx]))
      continue
    }
    if op == OP["true"] { push(st, true); continue }
    if op == OP["false"] { push(st, false); continue }
    if op == OP["null"] { push(st, null); continue }
    if op == OP.pop { pop(st); continue }
    if op == OP.get {
      idx = u16(code, ip)
      ip = ip + 2
      k = names[idx]
      v = env_get(env, k)
      if v == null {
        b = builtins()
        if k in b { v = b[k] } else { must({err: "identifier not found: " + k}) }
      }
      push(st, v)
      continue
    }
    if op == OP.set {
      idx = u16(code, ip)
      ip = ip + 2
      env_set(env, names[idx], peek(st))
      continue
    }
    if op == OP.add { r = pop(st); l = pop(st); push(st, l + r); continue }
    if op == OP.sub { r = pop(st); l = pop(st); push(st, l - r); continue }
    if op == OP.mul { r = pop(st); l = pop(st); push(st, l * r); continue }
    if op == OP.div { r = pop(st); l = pop(st); push(st, l / r); continue }
    if op == OP.mod { r = pop(st); l = pop(st); push(st, l % r); continue }
    if op == OP.eq { r = pop(st); l = pop(st); push(st, l == r); continue }
    if op == OP.neq { r = pop(st); l = pop(st); push(st, l != r); continue }
    if op == OP.lt { r = pop(st); l = pop(st); push(st, l < r); continue }
    if op == OP.lte { r = pop(st); l = pop(st); push(st, l <= r); continue }
    if op == OP.gt { r = pop(st); l = pop(st); push(st, l > r); continue }
    if op == OP.gte { r = pop(st); l = pop(st); push(st, l >= r); continue }
    if op == OP["in"] { r = pop(st); l = pop(st); push(st, l in r); continue }
    if op == OP.minus { push(st, -pop(st)); continue }
    if op == OP.bang { push(st, !(pop(st))); continue }
    if op == OP.jump { ip = u16(code, ip); continue }
    if op == OP.jump_false {
      t = u16(code, ip)
      ip = ip + 2
      if truthy(peek(st)) == false { ip = t }
      continue
    }
    if op == OP.jump_true {
      t = u16(code, ip)
      ip = ip + 2
      if truthy(peek(st)) { ip = t }
      continue
    }
    if op == OP.to_bool { push(st, truthy(pop(st))); continue }
    if op == OP.array {
      n = u16(code, ip)
      ip = ip + 2
      xs = []
      i = 0
      while i < n { xs = append(xs, null); i = i + 1 }
      i = n - 1
      while i >= 0 { xs[i] = pop(st); i = i - 1 }
      push(st, xs)
      continue
    }
    if op == OP.map {
      n = u16(code, ip)
      ip = ip + 2
      m = {}
      i = 0
      while i < n {
        v = pop(st)
        k = pop(st)
        m[k] = v
        i = i + 1
      }
      push(st, m)
      continue
    }
    if op == OP.index { idx = pop(st); left = pop(st); push(st, left[idx]); continue }
    if op == OP.set_index {
      idx = pop(st)
      obj = pop(st)
      obj[idx] = peek(st)
      continue
    }
    if op == OP.prop {
      idx = u16(code, ip)
      ip = ip + 2
      push(st, pop(st)[names[idx]])
      continue
    }
    if op == OP.set_prop {
      idx = u16(code, ip)
      ip = ip + 2
      obj = pop(st)
      obj[names[idx]] = peek(st)
      continue
    }
    if op == OP.call {
      n = u16(code, ip)
      ip = ip + 2
      args = []
      i = 0
      while i < n { args = append(args, null); i = i + 1 }
      i = n - 1
      while i >= 0 { args[i] = pop(st); i = i - 1 }
      f = pop(st)
      if type(f) == "MAP" && f.__fn {
        ex = {store: {}, outer: f.env, dot: env.dot}
        j = 0
        while j < len(f.params) {
          if j < len(args) { ex.store[f.params[j]] = args[j] }
          j = j + 1
        }
        push(st, run(f, ex))
      } else {
        push(st, apply(f, args))
      }
      continue
    }
    if op == OP["return"] { return pop(st) }
    if op == OP.closure {
      idx = u16(code, ip)
      ip = ip + 2
      tmpl = as_fn(consts[idx])
      push(st, {__fn: true, code: tmpl.code, consts: tmpl.consts, names: tmpl.names, params: tmpl.params ?? [], env: env})
      continue
    }
    if op == OP.dot { push(st, env.dot); continue }
    if op == OP.dot_field {
      idx = u16(code, ip)
      ip = ip + 2
      push(st, env.dot[names[idx]])
      continue
    }
    must({err: "unknown opcode " + str(op)})
  }
  if st.n == 0 { return null }
  peek(st)
}

{run: run}
