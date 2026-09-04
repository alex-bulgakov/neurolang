#define _CRT_SECURE_NO_WARNINGS
#include "nl.h"
#include <ctype.h>
#include <dirent.h>
#include <stdarg.h>
#include <errno.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <direct.h>
#define getcwd _getcwd
#else
#include <unistd.h>
#endif

#define VNULL ((NlVal){.t = T_NULL})
#define VBOOL(x) ((NlVal){.t = T_BOOL, .u.b = !!(x)})
#define VINT(x) ((NlVal){.t = T_INT, .u.i = (int64_t)(x)})
#define VFLOAT(x) ((NlVal){.t = T_FLOAT, .u.f = (double)(x)})

enum {
	OP_CONSTANT, OP_TRUE, OP_FALSE, OP_NULL, OP_POP, OP_GET, OP_SET,
	OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_EQ, OP_NEQ, OP_LT, OP_LTE, OP_GT, OP_GTE, OP_IN,
	OP_MINUS, OP_BANG, OP_JUMP, OP_JUMP_FALSE, OP_JUMP_TRUE, OP_JUMP_OK, OP_TO_BOOL,
	OP_ARRAY, OP_MAP, OP_INDEX, OP_SET_INDEX, OP_PROP, OP_SET_PROP,
	OP_CALL, OP_RETURN, OP_CLOSURE, OP_DOT, OP_DOT_FIELD,
	OP_PIPE_FILTER, OP_PIPE_MAP, OP_PIPE_REDUCE, OP_PIPE_CALL,
	OP_TOOL, OP_USE, OP_ITER, OP_ITER_NEXT, OP_SET_DOT, OP_BREAK, OP_CONTINUE
};

static const char *op_names[] = {
	"constant", "true", "false", "null", "pop", "get", "set",
	"add", "sub", "mul", "div", "mod", "eq", "neq", "lt", "lte", "gt", "gte", "in",
	"minus", "bang", "jump", "jump_false", "jump_true", "jump_ok", "to_bool",
	"array", "map", "index", "set_index", "prop", "set_prop",
	"call", "return", "closure", "dot", "dot_field",
	"pipe_filter", "pipe_map", "pipe_reduce", "pipe_call",
	"tool", "use", "iter", "iter_next", "set_dot", "break", "continue"
};
static const int nop = (int)(sizeof(op_names) / sizeof(op_names[0]));

struct NlStr { char *p; int n; };
struct NlList { NlVal *p; int n, cap; };
struct NlMap {
	char **k;
	NlVal *v;
	int n, cap;
};
struct NlFn {
	unsigned char *code;
	int ncode;
	NlVal *consts;
	int nconsts;
	char **names;
	int nnames;
	char **params;
	int nparams;
	NlEnv *env;
};
struct NlIter { NlVal *p; int n, i; };
struct NlEnv {
	NlMap *store;
	NlEnv *outer;
	NlVal dot;
	int has_dot;
	char *file;
	char *dir;
};

typedef struct { char *p; int n, cap; } Buf;
typedef NlVal (*NlPrim)(NlVal *a, int n, NlEnv *env);

typedef struct { const char *name; NlPrim fn; } PrimEnt;

static NlEnv *active;
static NlVal std_compiler;
static int booted;
static int apply_depth;
static char inspect_store[1 << 16];
static char std_found[1024];

static void *xmalloc(size_t n) {
	void *p = malloc(n ? n : 1);
	if (!p) {
		fputs("nl: oom\n", stderr);
		exit(1);
	}
	return p;
}
static void *xrealloc(void *p, size_t n) {
	void *q = realloc(p, n ? n : 1);
	if (!q) {
		fputs("nl: oom\n", stderr);
		exit(1);
	}
	return q;
}
static char *xstrdup(const char *s) {
	size_t n = strlen(s);
	char *p = xmalloc(n + 1);
	memcpy(p, s, n + 1);
	return p;
}

static NlVal mk_err(const char *fmt, ...);
static NlVal mk_str(const char *s);
static NlVal mk_str_n(const char *s, int n);
static int map_get(NlMap *m, const char *k, NlVal *out);
static void map_set(NlMap *m, const char *k, NlVal v);
static NlMap *map_new(void);
static NlVal map_val(NlMap *m);
static int map_has(NlMap *m, const char *k);
static NlVal infix(const char *op, NlVal l, NlVal r);
static NlVal index_get(NlVal left, NlVal idx);
static NlVal prop_get(NlVal left, const char *name);
static int equal(NlVal a, NlVal b);
static int truthy(NlVal v);
static const char *inspect_v(NlVal v);
static NlVal json_parse(const char *s, int n);
static char *json_stringify(NlVal v);
static NlVal chunk_to_fn(NlVal v);
static NlVal as_const(NlVal v);
static NlVal vm_exec(NlFn *fn, NlEnv *env);
static NlVal apply(NlVal fn, NlVal *args, int n, NlEnv *env);
static NlVal use_module(const char *path, NlEnv *from);
static NlVal load_into(const char *path, NlEnv *env);
static NlVal compile_src(const char *src);
static char *read_file(const char *path, int *out_n);
static int write_file(const char *path, const char *data, int n);
static char *resolve_path(const char *path, NlEnv *from);
static NlVal call_tool(const char *name, NlVal *args, int n, NlEnv *env);
static NlVal prim_call(int id, NlVal *args, int n, NlEnv *env);
static int prim_id(const char *name);
static NlVal builtins_map(void);
static NlVal opcodes_map(void);
static int utf8_cp_len(const char *s, int n);
static int utf8_off(const char *s, int n, int cp);
static int utf8_cp(const char *s, int n);

static NlVal mk_str_n(const char *s, int n) {
	NlStr *p = xmalloc(sizeof *p);
	p->n = n;
	p->p = xmalloc(n + 1);
	if (n) memcpy(p->p, s, n);
	p->p[n] = 0;
	NlVal v = {.t = T_STR, .u.s = p};
	return v;
}
static NlVal mk_str(const char *s) { return mk_str_n(s, (int)strlen(s)); }

static NlVal mk_err(const char *fmt, ...) {
	char b[1024];
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(b, sizeof b, fmt, ap);
	va_end(ap);
	NlVal v = {.t = T_ERR, .u.err = xstrdup(b)};
	return v;
}

static NlList *list_new(void) {
	NlList *l = xmalloc(sizeof *l);
	l->p = NULL;
	l->n = l->cap = 0;
	return l;
}
static NlVal list_val(NlList *l) {
	NlVal v = {.t = T_LIST, .u.l = l};
	return v;
}
static void list_push(NlList *l, NlVal x) {
	if (l->n >= l->cap) {
		l->cap = l->cap ? l->cap * 2 : 8;
		l->p = xrealloc(l->p, (size_t)l->cap * sizeof(NlVal));
	}
	l->p[l->n++] = x;
}

static NlMap *map_new(void) {
	NlMap *m = xmalloc(sizeof *m);
	m->k = NULL;
	m->v = NULL;
	m->n = m->cap = 0;
	return m;
}
static NlVal map_val(NlMap *m) {
	NlVal v = {.t = T_MAP, .u.m = m};
	return v;
}
static int map_find(NlMap *m, const char *k) {
	int i;
	for (i = 0; i < m->n; i++)
		if (strcmp(m->k[i], k) == 0) return i;
	return -1;
}
static int map_get(NlMap *m, const char *k, NlVal *out) {
	int i = map_find(m, k);
	if (i < 0) return 0;
	if (out) *out = m->v[i];
	return 1;
}
static int map_has(NlMap *m, const char *k) { return map_find(m, k) >= 0; }
static void map_set(NlMap *m, const char *k, NlVal v) {
	int i = map_find(m, k);
	if (i >= 0) {
		m->v[i] = v;
		return;
	}
	if (m->n >= m->cap) {
		m->cap = m->cap ? m->cap * 2 : 8;
		m->k = xrealloc(m->k, (size_t)m->cap * sizeof(char *));
		m->v = xrealloc(m->v, (size_t)m->cap * sizeof(NlVal));
	}
	m->k[m->n] = xstrdup(k);
	m->v[m->n] = v;
	m->n++;
}
static NlMap *map_copy(NlMap *m) {
	NlMap *c = map_new();
	int i;
	for (i = 0; i < m->n; i++) map_set(c, m->k[i], m->v[i]);
	return c;
}

static NlEnv *env_new(void) {
	NlEnv *e = xmalloc(sizeof *e);
	e->store = map_new();
	e->outer = NULL;
	e->dot = VNULL;
	e->has_dot = 0;
	e->file = e->dir = NULL;
	return e;
}
static NlEnv *env_enclosed(NlEnv *outer) {
	NlEnv *e = env_new();
	e->outer = outer;
	if (outer) {
		e->dot = outer->dot;
		e->has_dot = outer->has_dot;
		e->file = outer->file;
		e->dir = outer->dir;
	}
	return e;
}
static int env_get(NlEnv *e, const char *k, NlVal *out) {
	while (e) {
		if (map_get(e->store, k, out)) return 1;
		e = e->outer;
	}
	return 0;
}
static void env_set(NlEnv *e, const char *k, NlVal v) { map_set(e->store, k, v); }
static NlVal env_dot(NlEnv *e) {
	while (e) {
		if (e->has_dot) return e->dot;
		e = e->outer;
	}
	return VNULL;
}

static void buf_put(Buf *b, const char *s, int n) {
	if (n < 0) n = (int)strlen(s);
	if (b->n + n + 1 > b->cap) {
		int c = b->cap ? b->cap : 64;
		while (c < b->n + n + 1) c *= 2;
		b->p = xrealloc(b->p, c);
		b->cap = c;
	}
	memcpy(b->p + b->n, s, n);
	b->n += n;
	b->p[b->n] = 0;
}
static void buf_puts(Buf *b, const char *s) { buf_put(b, s, -1); }
static void buf_printf(Buf *b, const char *fmt, ...) {
	char t[1024];
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(t, sizeof t, fmt, ap);
	va_end(ap);
	buf_puts(b, t);
}

static int cmpstr(const void *a, const void *b) {
	return strcmp(*(char *const *)a, *(char *const *)b);
}

static void inspect_into(Buf *b, NlVal v);

static void inspect_into(Buf *b, NlVal v) {
	int i;
	switch (v.t) {
	case T_NULL: buf_puts(b, "null"); break;
	case T_BOOL: buf_puts(b, v.u.b ? "true" : "false"); break;
	case T_INT: buf_printf(b, "%lld", (long long)v.u.i); break;
	case T_FLOAT: buf_printf(b, "%g", v.u.f); break;
	case T_STR: {
		buf_puts(b, "\"");
		for (i = 0; i < v.u.s->n; i++) {
			char c = v.u.s->p[i];
			if (c == '\n') buf_puts(b, "\\n");
			else if (c == '\t') buf_puts(b, "\\t");
			else if (c == '\r') buf_puts(b, "\\r");
			else if (c == '\\' || c == '"') {
				char e[3] = {'\\', c, 0};
				buf_puts(b, e);
			} else {
				char e[2] = {c, 0};
				buf_puts(b, e);
			}
		}
		buf_puts(b, "\"");
		break;
	}
	case T_LIST:
		buf_puts(b, "[");
		for (i = 0; i < v.u.l->n; i++) {
			if (i) buf_puts(b, ", ");
			inspect_into(b, v.u.l->p[i]);
		}
		buf_puts(b, "]");
		break;
	case T_MAP: {
		char **ks = xmalloc((size_t)v.u.m->n * sizeof(char *));
		for (i = 0; i < v.u.m->n; i++) ks[i] = v.u.m->k[i];
		qsort(ks, (size_t)v.u.m->n, sizeof(char *), cmpstr);
		buf_puts(b, "{");
		for (i = 0; i < v.u.m->n; i++) {
			NlVal x;
			if (i) buf_puts(b, ", ");
			buf_puts(b, ks[i]);
			buf_puts(b, ": ");
			map_get(v.u.m, ks[i], &x);
			inspect_into(b, x);
		}
		buf_puts(b, "}");
		free(ks);
		break;
	}
	case T_FN: buf_puts(b, "<compiled>"); break;
	case T_BUILTIN: buf_puts(b, "builtin function"); break;
	case T_ERR: buf_printf(b, "Error: %s", v.u.err); break;
	case T_ITER: buf_puts(b, "<iter>"); break;
	}
}

static const char *inspect_v(NlVal v) {
	Buf b = {0};
	inspect_into(&b, v);
	int n = b.n < (int)sizeof(inspect_store) - 1 ? b.n : (int)sizeof(inspect_store) - 1;
	memcpy(inspect_store, b.p ? b.p : "", n);
	inspect_store[n] = 0;
	free(b.p);
	return inspect_store;
}

const char *nl_inspect(NlVal v) { return inspect_v(v); }

int nl_is_err(NlVal v) {
	NlVal e;
	if (v.t == T_ERR) return 1;
	if (v.t == T_MAP && map_get(v.u.m, "err", &e) && e.t != T_NULL) return 1;
	return 0;
}

const char *nl_format_err(NlVal v) {
	if (v.t == T_ERR) return inspect_v(v);
	if (v.t == T_MAP) {
		NlVal msg = VNULL, line = VNULL, col = VNULL;
		map_get(v.u.m, "err", &msg);
		map_get(v.u.m, "line", &line);
		map_get(v.u.m, "col", &col);
		const char *ms = msg.t == T_STR ? msg.u.s->p : inspect_v(msg);
		if (line.t == T_INT && line.u.i > 0) {
			snprintf(inspect_store, sizeof inspect_store, "Error: line %lld, col %lld: %s",
				(long long)line.u.i, (long long)(col.t == T_INT ? col.u.i : 0), ms);
			return inspect_store;
		}
		snprintf(inspect_store, sizeof inspect_store, "Error: %s", ms);
		return inspect_store;
	}
	return inspect_v(v);
}

static int is_num(NlVal v) { return v.t == T_INT || v.t == T_FLOAT; }
static double as_float(NlVal v) { return v.t == T_FLOAT ? v.u.f : (double)v.u.i; }

static int equal(NlVal a, NlVal b) {
	int i;
	if (a.t != b.t) {
		if (is_num(a) && is_num(b)) return as_float(a) == as_float(b);
		return 0;
	}
	switch (a.t) {
	case T_NULL: return 1;
	case T_BOOL: return a.u.b == b.u.b;
	case T_INT: return a.u.i == b.u.i;
	case T_FLOAT: return a.u.f == b.u.f;
	case T_STR: return a.u.s->n == b.u.s->n && memcmp(a.u.s->p, b.u.s->p, a.u.s->n) == 0;
	case T_LIST:
		if (a.u.l->n != b.u.l->n) return 0;
		for (i = 0; i < a.u.l->n; i++)
			if (!equal(a.u.l->p[i], b.u.l->p[i])) return 0;
		return 1;
	case T_MAP:
		if (a.u.m->n != b.u.m->n) return 0;
		for (i = 0; i < a.u.m->n; i++) {
			NlVal x;
			if (!map_get(b.u.m, a.u.m->k[i], &x) || !equal(a.u.m->v[i], x)) return 0;
		}
		return 1;
	case T_FN: return a.u.fn == b.u.fn;
	case T_BUILTIN: return a.u.builtin == b.u.builtin;
	default: return 0;
	}
}

static int truthy(NlVal v) {
	switch (v.t) {
	case T_NULL: return 0;
	case T_BOOL: return v.u.b;
	case T_INT: return v.u.i != 0;
	case T_FLOAT: return v.u.f != 0;
	case T_STR: return v.u.s->n > 0;
	case T_LIST: return v.u.l->n > 0;
	default: return 1;
	}
}

static const char *str_raw(NlVal v) {
	if (v.t == T_STR) return v.u.s->p;
	return inspect_v(v);
}

static NlVal infix(const char *op, NlVal l, NlVal r) {
	if (strcmp(op, "in") == 0) {
		int i;
		if (r.t == T_LIST) {
			for (i = 0; i < r.u.l->n; i++)
				if (equal(l, r.u.l->p[i])) return VBOOL(1);
			return VBOOL(0);
		}
		if (r.t == T_MAP) return VBOOL(map_has(r.u.m, str_raw(l)));
		if (r.t == T_STR) {
			if (l.t != T_STR) return mk_err("in: left operand must be STRING when searching a STRING");
			return VBOOL(strstr(r.u.s->p, l.u.s->p) != NULL);
		}
		return mk_err("in: cannot search in %s", inspect_v(r));
	}
	if (strcmp(op, "==") == 0) return VBOOL(equal(l, r));
	if (strcmp(op, "!=") == 0) return VBOOL(!equal(l, r));
	if (is_num(l) && is_num(r)) {
		if (l.t == T_FLOAT || r.t == T_FLOAT) {
			double a = as_float(l), b = as_float(r);
			if (strcmp(op, "+") == 0) return VFLOAT(a + b);
			if (strcmp(op, "-") == 0) return VFLOAT(a - b);
			if (strcmp(op, "*") == 0) return VFLOAT(a * b);
			if (strcmp(op, "/") == 0) {
				if (b == 0) return mk_err("division by zero");
				return VFLOAT(a / b);
			}
			if (strcmp(op, "<") == 0) return VBOOL(a < b);
			if (strcmp(op, "<=") == 0) return VBOOL(a <= b);
			if (strcmp(op, ">") == 0) return VBOOL(a > b);
			if (strcmp(op, ">=") == 0) return VBOOL(a >= b);
			return mk_err("unknown operator");
		}
		int64_t a = l.u.i, b = r.u.i;
		if (strcmp(op, "+") == 0) return VINT(a + b);
		if (strcmp(op, "-") == 0) return VINT(a - b);
		if (strcmp(op, "*") == 0) return VINT(a * b);
		if (strcmp(op, "/") == 0) {
			if (b == 0) return mk_err("division by zero");
			return VINT(a / b);
		}
		if (strcmp(op, "%") == 0) {
			if (b == 0) return mk_err("modulo by zero");
			return VINT(a % b);
		}
		if (strcmp(op, "<") == 0) return VBOOL(a < b);
		if (strcmp(op, "<=") == 0) return VBOOL(a <= b);
		if (strcmp(op, ">") == 0) return VBOOL(a > b);
		if (strcmp(op, ">=") == 0) return VBOOL(a >= b);
		return mk_err("unknown operator");
	}
	if (l.t == T_STR && strcmp(op, "+") == 0) {
		const char *rs = r.t == T_STR ? r.u.s->p : inspect_v(r);
		Buf b = {0};
		buf_put(&b, l.u.s->p, l.u.s->n);
		buf_puts(&b, rs);
		NlVal o = mk_str_n(b.p, b.n);
		free(b.p);
		return o;
	}
	if (l.t == T_LIST && strcmp(op, "+") == 0) {
		NlList *o = list_new();
		int i;
		for (i = 0; i < l.u.l->n; i++) list_push(o, l.u.l->p[i]);
		if (r.t == T_LIST)
			for (i = 0; i < r.u.l->n; i++) list_push(o, r.u.l->p[i]);
		else
			list_push(o, r);
		return list_val(o);
	}
	return mk_err("type mismatch: %s %s %s", inspect_v(l), op, inspect_v(r));
}

static NlVal index_get(NlVal left, NlVal idx) {
	if (left.t == T_LIST && idx.t == T_INT) {
		if (idx.u.i < 0 || idx.u.i >= left.u.l->n) return VNULL;
		return left.u.l->p[idx.u.i];
	}
	if (left.t == T_MAP && idx.t == T_STR) {
		NlVal o;
		if (!map_get(left.u.m, idx.u.s->p, &o)) return VNULL;
		return o;
	}
	if (left.t == T_STR && idx.t == T_INT) {
		int ncp = utf8_cp_len(left.u.s->p, left.u.s->n);
		int off, end;
		if (idx.u.i < 0 || idx.u.i >= ncp) return VNULL;
		off = utf8_off(left.u.s->p, left.u.s->n, (int)idx.u.i);
		end = utf8_off(left.u.s->p, left.u.s->n, (int)idx.u.i + 1);
		return mk_str_n(left.u.s->p + off, end - off);
	}
	return mk_err("index operator not supported");
}

static NlVal prop_get(NlVal left, const char *name) {
	NlVal o;
	if (left.t == T_ERR) {
		NlMap *m = map_new();
		map_set(m, "err", mk_str(left.u.err));
		left = map_val(m);
	}
	if (left.t != T_MAP) return VNULL;
	if (!map_get(left.u.m, name, &o)) return VNULL;
	return o;
}

static NlVal prefix_minus(NlVal r) {
	if (r.t == T_INT) return VINT(-r.u.i);
	if (r.t == T_FLOAT) return VFLOAT(-r.u.f);
	return mk_err("unknown operator: -%s", inspect_v(r));
}
static NlVal prefix_bang(NlVal r) { return VBOOL(!truthy(r)); }

/* ---------- JSON ---------- */
typedef struct { const char *s; int i, n; char err[256]; } JS;

static int js_ok(JS *j) { return j->err[0] == 0; }
static void js_skip(JS *j) {
	while (j->i < j->n && (unsigned char)j->s[j->i] <= 32) j->i++;
}
static int js_eat(JS *j, char c) {
	js_skip(j);
	if (j->i < j->n && j->s[j->i] == c) {
		j->i++;
		return 1;
	}
	return 0;
}

static NlVal js_val(JS *j);

static NlVal js_str(JS *j) {
	Buf b = {0};
	j->i++;
	while (j->i < j->n) {
		unsigned char c = (unsigned char)j->s[j->i++];
		if (c == '"') {
			NlVal v = mk_str_n(b.p ? b.p : "", b.n);
			free(b.p);
			return v;
		}
		if (c == '\\' && j->i < j->n) {
			char e = j->s[j->i++];
			if (e == 'n') e = '\n';
			else if (e == 't') e = '\t';
			else if (e == 'r') e = '\r';
			else if (e == 'u' && j->i + 4 <= j->n) {
				unsigned cp = 0;
				int k;
				for (k = 0; k < 4; k++) {
					char h = j->s[j->i++];
					cp <<= 4;
					if (h >= '0' && h <= '9') cp += h - '0';
					else if (h >= 'a' && h <= 'f') cp += h - 'a' + 10;
					else if (h >= 'A' && h <= 'F') cp += h - 'A' + 10;
				}
				if (cp < 0x80) {
					char ch = (char)cp;
					buf_put(&b, &ch, 1);
				} else if (cp < 0x800) {
					char u[2] = {(char)(0xC0 | (cp >> 6)), (char)(0x80 | (cp & 0x3F))};
					buf_put(&b, u, 2);
				} else {
					char u[3] = {(char)(0xE0 | (cp >> 12)), (char)(0x80 | ((cp >> 6) & 0x3F)), (char)(0x80 | (cp & 0x3F))};
					buf_put(&b, u, 3);
				}
				continue;
			}
			buf_put(&b, &e, 1);
		} else {
			char ch = (char)c;
			buf_put(&b, &ch, 1);
		}
	}
	free(b.p);
	snprintf(j->err, sizeof j->err, "unterminated string");
	return VNULL;
}

static NlVal js_num(JS *j) {
	int start = j->i, is_f = 0;
	if (j->s[j->i] == '-') j->i++;
	while (j->i < j->n && isdigit((unsigned char)j->s[j->i])) j->i++;
	if (j->i < j->n && j->s[j->i] == '.') {
		is_f = 1;
		j->i++;
		while (j->i < j->n && isdigit((unsigned char)j->s[j->i])) j->i++;
	}
	if (j->i < j->n && (j->s[j->i] == 'e' || j->s[j->i] == 'E')) {
		is_f = 1;
		j->i++;
		if (j->i < j->n && (j->s[j->i] == '+' || j->s[j->i] == '-')) j->i++;
		while (j->i < j->n && isdigit((unsigned char)j->s[j->i])) j->i++;
	}
	char tmp[128];
	int n = j->i - start;
	if (n >= (int)sizeof tmp) n = (int)sizeof tmp - 1;
	memcpy(tmp, j->s + start, n);
	tmp[n] = 0;
	if (!is_f) {
		long long x = strtoll(tmp, NULL, 10);
		return VINT(x);
	}
	return VFLOAT(strtod(tmp, NULL));
}

static NlVal js_val(JS *j) {
	js_skip(j);
	if (!js_ok(j) || j->i >= j->n) {
		snprintf(j->err, sizeof j->err, "unexpected end");
		return VNULL;
	}
	char c = j->s[j->i];
	if (c == '"') return js_str(j);
	if (c == '-' || isdigit((unsigned char)c)) return js_num(j);
	if (c == 't' && j->i + 4 <= j->n && !memcmp(j->s + j->i, "true", 4)) {
		j->i += 4;
		return VBOOL(1);
	}
	if (c == 'f' && j->i + 5 <= j->n && !memcmp(j->s + j->i, "false", 5)) {
		j->i += 5;
		return VBOOL(0);
	}
	if (c == 'n' && j->i + 4 <= j->n && !memcmp(j->s + j->i, "null", 4)) {
		j->i += 4;
		return VNULL;
	}
	if (c == '[') {
		NlList *l = list_new();
		j->i++;
		js_skip(j);
		if (js_eat(j, ']')) return list_val(l);
		for (;;) {
			list_push(l, js_val(j));
			if (!js_ok(j)) return VNULL;
			js_skip(j);
			if (js_eat(j, ']')) return list_val(l);
			if (!js_eat(j, ',')) {
				snprintf(j->err, sizeof j->err, "expected comma");
				return VNULL;
			}
		}
	}
	if (c == '{') {
		NlMap *m = map_new();
		j->i++;
		js_skip(j);
		if (js_eat(j, '}')) return map_val(m);
		for (;;) {
			js_skip(j);
			if (j->s[j->i] != '"') {
				snprintf(j->err, sizeof j->err, "expected key");
				return VNULL;
			}
			NlVal k = js_str(j);
			if (!js_ok(j) || k.t != T_STR) return VNULL;
			js_skip(j);
			if (!js_eat(j, ':')) {
				snprintf(j->err, sizeof j->err, "expected colon");
				return VNULL;
			}
			map_set(m, k.u.s->p, js_val(j));
			if (!js_ok(j)) return VNULL;
			js_skip(j);
			if (js_eat(j, '}')) return map_val(m);
			if (!js_eat(j, ',')) {
				snprintf(j->err, sizeof j->err, "expected comma");
				return VNULL;
			}
		}
	}
	snprintf(j->err, sizeof j->err, "bad json at %d", j->i);
	return VNULL;
}

static NlVal json_parse(const char *s, int n) {
	JS j = {.s = s, .i = 0, .n = n, .err = {0}};
	NlVal v = js_val(&j);
	if (j.err[0]) return mk_err("json: %s", j.err);
	return v;
}

static void json_into(Buf *b, NlVal v) {
	int i;
	switch (v.t) {
	case T_NULL: buf_puts(b, "null"); break;
	case T_BOOL: buf_puts(b, v.u.b ? "true" : "false"); break;
	case T_INT: buf_printf(b, "%lld", (long long)v.u.i); break;
	case T_FLOAT: buf_printf(b, "%g", v.u.f); break;
	case T_STR: inspect_into(b, v); break;
	case T_LIST:
		buf_puts(b, "[");
		for (i = 0; i < v.u.l->n; i++) {
			if (i) buf_puts(b, ",");
			json_into(b, v.u.l->p[i]);
		}
		buf_puts(b, "]");
		break;
	case T_MAP:
		buf_puts(b, "{");
		for (i = 0; i < v.u.m->n; i++) {
			if (i) buf_puts(b, ",");
			inspect_into(b, mk_str(v.u.m->k[i]));
			buf_puts(b, ":");
			json_into(b, v.u.m->v[i]);
		}
		buf_puts(b, "}");
		break;
	default:
		buf_puts(b, "null");
		break;
	}
}
static char *json_stringify(NlVal v) {
	Buf b = {0};
	json_into(&b, v);
	return b.p ? b.p : xstrdup("null");
}

static NlFn *fn_new(void) {
	NlFn *f = xmalloc(sizeof *f);
	memset(f, 0, sizeof *f);
	return f;
}
static NlVal fn_val(NlFn *f) {
	NlVal v = {.t = T_FN, .u.fn = f};
	return v;
}

static int list_ints(NlVal v, unsigned char **out, int *n) {
	int i;
	if (v.t != T_LIST) return 0;
	*n = v.u.l->n;
	*out = xmalloc((size_t)(*n ? *n : 1));
	for (i = 0; i < v.u.l->n; i++) {
		NlVal e = v.u.l->p[i];
		int x = 0;
		if (e.t == T_INT) x = (int)e.u.i;
		else if (e.t == T_FLOAT) x = (int)e.u.f;
		(*out)[i] = (unsigned char)x;
	}
	return 1;
}

static char **list_strs(NlVal v, int *n) {
	int i;
	char **a;
	if (v.t != T_LIST) {
		*n = 0;
		return NULL;
	}
	*n = v.u.l->n;
	a = xmalloc((size_t)(*n ? *n : 1) * sizeof(char *));
	for (i = 0; i < v.u.l->n; i++) {
		NlVal e = v.u.l->p[i];
		a[i] = xstrdup(e.t == T_STR ? e.u.s->p : inspect_v(e));
	}
	return a;
}

static NlVal chunk_to_fn(NlVal v) {
	NlVal code = VNULL, consts = VNULL, names = VNULL, params = VNULL;
	NlFn *f;
	int i;
	if (v.t == T_FN) return v;
	if (v.t != T_MAP) return mk_err("chunk must be a map");
	map_get(v.u.m, "code", &code);
	map_get(v.u.m, "consts", &consts);
	map_get(v.u.m, "names", &names);
	map_get(v.u.m, "params", &params);
	f = fn_new();
	if (!list_ints(code, &f->code, &f->ncode)) {
		f->code = xmalloc(1);
		f->ncode = 0;
	}
	if (consts.t == T_LIST) {
		f->nconsts = consts.u.l->n;
		f->consts = xmalloc((size_t)(f->nconsts ? f->nconsts : 1) * sizeof(NlVal));
		for (i = 0; i < f->nconsts; i++) f->consts[i] = as_const(consts.u.l->p[i]);
	}
	f->names = list_strs(names, &f->nnames);
	f->params = list_strs(params, &f->nparams);
	return fn_val(f);
}

static NlVal as_const(NlVal v) {
	int i;
	if (v.t == T_MAP && (map_has(v.u.m, "__bc") || map_has(v.u.m, "code"))) return chunk_to_fn(v);
	if (v.t == T_LIST) {
		for (i = 0; i < v.u.l->n; i++) v.u.l->p[i] = as_const(v.u.l->p[i]);
	}
	return v;
}

static NlVal read_nlc(const char *path) {
	int n = 0;
	char *s = read_file(path, &n);
	NlVal v;
	if (!s) return VNULL;
	v = json_parse(s, n);
	free(s);
	if (v.t == T_ERR) return VNULL;
	return v;
}

static char *nlc_path(const char *nl) {
	int n = (int)strlen(nl);
	char *p = xmalloc(n + 4);
	if (n >= 3 && strcmp(nl + n - 3, ".nl") == 0) {
		memcpy(p, nl, n - 3);
		memcpy(p + n - 3, ".nlc", 5);
	} else {
		memcpy(p, nl, n);
		memcpy(p + n, ".nlc", 5);
	}
	return p;
}

static char *read_file(const char *path, int *out_n) {
	FILE *f = fopen(path, "rb");
	long sz;
	char *s;
	if (!f) return NULL;
	fseek(f, 0, SEEK_END);
	sz = ftell(f);
	fseek(f, 0, SEEK_SET);
	if (sz < 0) sz = 0;
	s = xmalloc((size_t)sz + 1);
	if (sz) fread(s, 1, (size_t)sz, f);
	s[sz] = 0;
	fclose(f);
	if (out_n) *out_n = (int)sz;
	return s;
}
static int write_file(const char *path, const char *data, int n) {
	FILE *f = fopen(path, "wb");
	if (!f) return 0;
	if (n) fwrite(data, 1, (size_t)n, f);
	fclose(f);
	return 1;
}

static int file_mtime(const char *p, time_t *t) {
	struct stat st;
	if (stat(p, &st) != 0) return 0;
	*t = st.st_mtime;
	return 1;
}

static char *join_path(const char *a, const char *b) {
	int na = (int)strlen(a), nb = (int)strlen(b);
	char *p = xmalloc(na + nb + 2);
	memcpy(p, a, na);
	if (na && a[na - 1] != '/' && a[na - 1] != '\\') {
		p[na] = '/';
		memcpy(p + na + 1, b, nb + 1);
	} else {
		memcpy(p + na, b, nb + 1);
	}
	return p;
}

static int exists_file(const char *p) {
	struct stat st;
	return stat(p, &st) == 0 && !(st.st_mode & S_IFDIR);
}

static char *dir_of(const char *p) {
	char *d = xstrdup(p);
	int i, n = (int)strlen(d);
	for (i = n - 1; i >= 0; i--)
		if (d[i] == '/' || d[i] == '\\') {
			d[i] = 0;
			return d;
		}
	d[0] = '.';
	d[1] = 0;
	return d;
}

static char *abspath(const char *p) {
#ifdef _WIN32
	char buf[MAX_PATH];
	if (!_fullpath(buf, p, MAX_PATH)) return xstrdup(p);
	return xstrdup(buf);
#else
	char buf[4096];
	if (!realpath(p, buf)) return xstrdup(p);
	return xstrdup(buf);
#endif
}

static int has_gomod(const char *dir) {
	char *p = join_path(dir, "go.mod");
	int ok = exists_file(p);
	free(p);
	return ok;
}

char *resolve_path(const char *path, NlEnv *from) {
	char *cands[64];
	int nc = 0, i;
	char cwd[4096];
	const char *vars[2];
	int nv = 0;
	char *with_nl = NULL;
	vars[nv++] = path;
	if (strlen(path) < 3 || strcmp(path + strlen(path) - 3, ".nl") != 0) {
		with_nl = xmalloc(strlen(path) + 4);
		sprintf(with_nl, "%s.nl", path);
		vars[nv++] = with_nl;
	}
	getcwd(cwd, sizeof cwd);
	for (i = 0; i < nv; i++) {
		const char *v = vars[i];
		if (from && from->dir) cands[nc++] = join_path(from->dir, v);
		cands[nc++] = join_path(cwd, v);
		{
			char *d = xstrdup(cwd);
			for (;;) {
				cands[nc++] = join_path(d, v);
				if (has_gomod(d)) break;
				{
					char *parent = dir_of(d);
					if (strcmp(parent, d) == 0) {
						free(parent);
						break;
					}
					free(d);
					d = parent;
				}
			}
			free(d);
		}
	}
	for (i = 0; i < nc; i++) {
		char *abs = abspath(cands[i]);
		if (exists_file(abs)) {
			int k;
			for (k = 0; k < nc; k++) free(cands[k]);
			free(with_nl);
			return abs;
		}
		free(abs);
	}
	for (i = 0; i < nc; i++) free(cands[i]);
	free(with_nl);
	return NULL;
}

static NlVal load_fresh_nlc(const char *nl) {
	char *np = nlc_path(nl);
	time_t ts = 0, tc = 0;
	NlVal v = VNULL;
	if (!file_mtime(nl, &ts) || !file_mtime(np, &tc) || tc < ts) {
		free(np);
		return VNULL;
	}
	v = read_nlc(np);
	free(np);
	return v.t == T_NULL ? VNULL : v;
}

static NlVal run_module(NlVal bc, const char *resolved, NlVal env_map) {
	NlEnv *prev = active;
	NlEnv *tmp = env_new();
	NlVal r;
	tmp->file = xstrdup(resolved);
	tmp->dir = dir_of(resolved);
	active = tmp;
	r = nl_vm_run(bc, env_map);
	active = prev;
	return r;
}

static NlVal eval_module(const char *src, const char *resolved, NlVal env_map, int cache) {
	NlVal bc = load_fresh_nlc(resolved);
	if (bc.t == T_NULL && !booted) bc = read_nlc(nlc_path(resolved));
	if (bc.t == T_NULL) {
		bc = compile_src(src);
		if (nl_is_err(bc)) return bc;
		if (cache) {
			char *js = json_stringify(bc);
			char *np = nlc_path(resolved);
			write_file(np, js, (int)strlen(js));
			free(js);
			free(np);
		}
	}
	return run_module(bc, resolved, env_map);
}

static NlVal use_module(const char *path, NlEnv *from) {
	char *res = resolve_path(path, from);
	int n = 0;
	char *src;
	NlVal r, env_map;
	if (!res) return mk_err("use: module not found: %s", path);
	src = read_file(res, &n);
	if (!src) {
		free(res);
		return mk_err("use: cannot read %s", path);
	}
	env_map = map_val(map_new());
	r = eval_module(src, res, env_map, 1);
	free(src);
	free(res);
	if (nl_is_err(r)) return r;
	if (r.t == T_MAP) return r;
	return env_map;
}

static NlVal load_into(const char *path, NlEnv *env) {
	char *res = resolve_path(path, env);
	int n = 0;
	char *src;
	NlVal env_map, r;
	char *pf, *pd;
	if (!res) return mk_err("load: module not found: %s", path);
	src = read_file(res, &n);
	if (!src) {
		free(res);
		return mk_err("load error: cannot read");
	}
	env_map = map_val(map_copy(env->store));
	pf = env->file;
	pd = env->dir;
	env->file = res;
	env->dir = dir_of(res);
	r = eval_module(src, res, env_map, 1);
	env->file = pf;
	env->dir = pd;
	if (env_map.t == T_MAP) {
		int i;
		for (i = 0; i < env_map.u.m->n; i++) env_set(env, env_map.u.m->k[i], env_map.u.m->v[i]);
	}
	free(src);
	return r;
}

static NlVal compile_src(const char *src) {
	NlVal parse, compile, ast, bc, stmts, first, typ;
	if (std_compiler.t != T_MAP) return mk_err("std compiler is not booted");
	if (!map_get(std_compiler.u.m, "nl_parse", &parse) || !map_get(std_compiler.u.m, "nl_compile", &compile))
		return mk_err("std compiler is missing nl_parse/nl_compile");
	{
		NlVal a[1] = {mk_str(src)};
		ast = apply(parse, a, 1, active);
	}
	if (nl_is_err(ast)) return ast;
	if (ast.t == T_MAP && map_get(ast.u.m, "statements", &stmts) && stmts.t == T_LIST && stmts.u.l->n > 0) {
		first = stmts.u.l->p[0];
		if (first.t == T_MAP && map_get(first.u.m, "type", &typ) && typ.t == T_STR && strcmp(typ.u.s->p, "Err") == 0) {
			NlVal msg = VNULL, line = VNULL, col = VNULL;
			NlMap *em = map_new();
			map_get(first.u.m, "msg", &msg);
			map_get(first.u.m, "line", &line);
			map_get(first.u.m, "col", &col);
			map_set(em, "err", msg.t == T_STR ? msg : mk_str("parse"));
			if (line.t == T_INT) map_set(em, "line", line);
			if (col.t == T_INT) map_set(em, "col", col);
			return mk_err("%s", nl_format_err(map_val(em)));
		}
	}
	{
		NlVal a[1] = {ast};
		bc = apply(compile, a, 1, active);
	}
	return bc;
}

static int utf8_cp_len(const char *s, int n) {
	int i = 0, cnt = 0;
	while (i < n) {
		unsigned char c = (unsigned char)s[i];
		if (c < 0x80) i++;
		else if ((c & 0xE0) == 0xC0) i += 2;
		else if ((c & 0xF0) == 0xE0) i += 3;
		else i += 4;
		cnt++;
	}
	return cnt;
}
static int utf8_off(const char *s, int n, int cp) {
	int i = 0, k = 0;
	if (cp <= 0) return 0;
	while (i < n && k < cp) {
		unsigned char c = (unsigned char)s[i];
		if (c < 0x80) i++;
		else if ((c & 0xE0) == 0xC0) i += 2;
		else if ((c & 0xF0) == 0xE0) i += 3;
		else i += 4;
		k++;
	}
	return i;
}
static int utf8_cp(const char *s, int n) {
	unsigned char c;
	if (n <= 0) return 0;
	c = (unsigned char)s[0];
	if (c < 0x80) return c;
	if ((c & 0xE0) == 0xC0 && n >= 2) return ((c & 0x1F) << 6) | ((unsigned char)s[1] & 0x3F);
	if ((c & 0xF0) == 0xE0 && n >= 3)
		return ((c & 0x0F) << 12) | (((unsigned char)s[1] & 0x3F) << 6) | ((unsigned char)s[2] & 0x3F);
	return c;
}

static NlVal prim_len(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return mk_err("wrong number of arguments. got=%d, want=1", n);
	if (a[0].t == T_STR) return VINT(utf8_cp_len(a[0].u.s->p, a[0].u.s->n));
	if (a[0].t == T_LIST) return VINT(a[0].u.l->n);
	if (a[0].t == T_MAP) return VINT(a[0].u.m->n);
	return mk_err("argument to `len` not supported");
}
static NlVal prim_print(NlVal *a, int n, NlEnv *env) {
	int i;
	(void)env;
	for (i = 0; i < n; i++) {
		if (i) putchar(' ');
		if (a[i].t == T_STR) fputs(a[i].u.s->p, stdout);
		else fputs(inspect_v(a[i]), stdout);
	}
	putchar('\n');
	return n == 1 ? a[0] : VNULL;
}
static NlVal prim_type(NlVal *a, int n, NlEnv *env) {
	const char *t = "NULL";
	(void)env;
	if (n != 1) return mk_err("wrong number of arguments. got=%d, want=1", n);
	switch (a[0].t) {
	case T_INT: t = "INTEGER"; break;
	case T_FLOAT: t = "FLOAT"; break;
	case T_BOOL: t = "BOOLEAN"; break;
	case T_STR: t = "STRING"; break;
	case T_LIST: t = "LIST"; break;
	case T_MAP: t = "MAP"; break;
	case T_FN:
	case T_BUILTIN: t = "FUNCTION"; break;
	case T_ERR: t = "ERROR"; break;
	default: break;
	}
	return mk_str(t);
}
static NlVal prim_range(NlVal *a, int n, NlEnv *env) {
	int64_t start = 0, end = 0, i;
	NlList *l;
	(void)env;
	if (n == 1 && a[0].t == T_INT) end = a[0].u.i;
	else if (n == 2 && a[0].t == T_INT && a[1].t == T_INT) {
		start = a[0].u.i;
		end = a[1].u.i;
	} else
		return mk_err("wrong number of arguments to range");
	l = list_new();
	for (i = start; i < end; i++) list_push(l, VINT(i));
	return list_val(l);
}
static NlVal prim_keys(NlVal *a, int n, NlEnv *env) {
	char **ks;
	int i;
	NlList *l;
	(void)env;
	if (n != 1 || a[0].t != T_MAP) return mk_err("argument to keys must be MAP");
	ks = xmalloc((size_t)a[0].u.m->n * sizeof(char *));
	for (i = 0; i < a[0].u.m->n; i++) ks[i] = a[0].u.m->k[i];
	qsort(ks, (size_t)a[0].u.m->n, sizeof(char *), cmpstr);
	l = list_new();
	for (i = 0; i < a[0].u.m->n; i++) list_push(l, mk_str(ks[i]));
	free(ks);
	return list_val(l);
}
static NlVal prim_values(NlVal *a, int n, NlEnv *env) {
	int i;
	NlList *l;
	(void)env;
	if (n != 1 || a[0].t != T_MAP) return mk_err("argument to values must be MAP");
	l = list_new();
	for (i = 0; i < a[0].u.m->n; i++) list_push(l, a[0].u.m->v[i]);
	return list_val(l);
}
static NlVal prim_join(NlVal *a, int n, NlEnv *env) {
	Buf b = {0};
	int i;
	(void)env;
	if (n != 2 || a[0].t != T_LIST || a[1].t != T_STR) return mk_err("join expects (LIST, STRING)");
	for (i = 0; i < a[0].u.l->n; i++) {
		if (i) buf_put(&b, a[1].u.s->p, a[1].u.s->n);
		if (a[0].u.l->p[i].t == T_STR) buf_put(&b, a[0].u.l->p[i].u.s->p, a[0].u.l->p[i].u.s->n);
		else buf_puts(&b, inspect_v(a[0].u.l->p[i]));
	}
	{
		NlVal o = mk_str_n(b.p ? b.p : "", b.n);
		free(b.p);
		return o;
	}
}
static NlVal prim_split(NlVal *a, int n, NlEnv *env) {
	NlList *l;
	const char *s, *sep, *p;
	(void)env;
	if (n != 2 || a[0].t != T_STR || a[1].t != T_STR) return mk_err("split expects (STRING, STRING)");
	l = list_new();
	s = a[0].u.s->p;
	sep = a[1].u.s->p;
	if (!sep[0]) {
		int i;
		for (i = 0; i < a[0].u.s->n; i++) list_push(l, mk_str_n(s + i, 1));
		return list_val(l);
	}
	while ((p = strstr(s, sep))) {
		list_push(l, mk_str_n(s, (int)(p - s)));
		s = p + strlen(sep);
	}
	list_push(l, mk_str(s));
	return list_val(l);
}
static NlVal prim_slice(NlVal *a, int n, NlEnv *env) {
	int start, end, len, i;
	(void)env;
	if (n < 2 || n > 3 || a[1].t != T_INT) return mk_err("slice expects (seq, start, [end])");
	start = (int)a[1].u.i;
	if (a[0].t == T_STR) {
		int ncp = utf8_cp_len(a[0].u.s->p, a[0].u.s->n);
		int sb, eb;
		end = n == 3 && a[2].t == T_INT ? (int)a[2].u.i : ncp;
		if (start < 0) start = 0;
		if (start > ncp) start = ncp;
		if (end < start) end = start;
		if (end > ncp) end = ncp;
		sb = utf8_off(a[0].u.s->p, a[0].u.s->n, start);
		eb = utf8_off(a[0].u.s->p, a[0].u.s->n, end);
		return mk_str_n(a[0].u.s->p + sb, eb - sb);
	}
	if (a[0].t == T_LIST) {
		NlList *o;
		len = a[0].u.l->n;
		end = n == 3 && a[2].t == T_INT ? (int)a[2].u.i : len;
		if (start < 0) start = 0;
		if (start > len) start = len;
		if (end < start) end = start;
		if (end > len) end = len;
		o = list_new();
		for (i = start; i < end; i++) list_push(o, a[0].u.l->p[i]);
		return list_val(o);
	}
	return mk_err("slice expects STRING or LIST");
}
static NlVal prim_append(NlVal *a, int n, NlEnv *env) {
	NlList *o;
	int i;
	(void)env;
	if (n != 2 || a[0].t != T_LIST) return mk_err("append expects (LIST, elem)");
	o = list_new();
	for (i = 0; i < a[0].u.l->n; i++) list_push(o, a[0].u.l->p[i]);
	list_push(o, a[1]);
	return list_val(o);
}
static NlVal prim_ord(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1 || a[0].t != T_STR || a[0].u.s->n == 0) return mk_err("ord expects non-empty STRING");
	return VINT(utf8_cp(a[0].u.s->p, a[0].u.s->n));
}
static NlVal prim_chr(NlVal *a, int n, NlEnv *env) {
	int cp;
	char u[5];
	int k = 0;
	(void)env;
	if (n != 1 || a[0].t != T_INT) return mk_err("chr expects INTEGER");
	cp = (int)a[0].u.i;
	if (cp < 0x80) u[k++] = (char)cp;
	else if (cp < 0x800) {
		u[k++] = (char)(0xC0 | (cp >> 6));
		u[k++] = (char)(0x80 | (cp & 0x3F));
	} else {
		u[k++] = (char)(0xE0 | (cp >> 12));
		u[k++] = (char)(0x80 | ((cp >> 6) & 0x3F));
		u[k++] = (char)(0x80 | (cp & 0x3F));
	}
	return mk_str_n(u, k);
}
static NlVal prim_is_digit(NlVal *a, int n, NlEnv *env) {
	int cp;
	(void)env;
	if (n != 1 || a[0].t != T_STR || a[0].u.s->n == 0) return VBOOL(0);
	cp = utf8_cp(a[0].u.s->p, a[0].u.s->n);
	return VBOOL(cp >= '0' && cp <= '9');
}
static NlVal prim_is_alpha(NlVal *a, int n, NlEnv *env) {
	int cp;
	(void)env;
	if (n != 1 || a[0].t != T_STR || a[0].u.s->n == 0) return VBOOL(0);
	cp = utf8_cp(a[0].u.s->p, a[0].u.s->n);
	return VBOOL((cp >= 'a' && cp <= 'z') || (cp >= 'A' && cp <= 'Z') || cp == '_' || cp == '$');
}
static NlVal prim_is_space(NlVal *a, int n, NlEnv *env) {
	int cp;
	(void)env;
	if (n != 1 || a[0].t != T_STR || a[0].u.s->n == 0) return VBOOL(0);
	cp = utf8_cp(a[0].u.s->p, a[0].u.s->n);
	return VBOOL(cp == ' ' || cp == '\t' || cp == '\n' || cp == '\r');
}
static NlVal prim_int(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return mk_err("int expects 1 argument");
	if (a[0].t == T_INT) return a[0];
	if (a[0].t == T_FLOAT) return VINT((int64_t)a[0].u.f);
	if (a[0].t == T_BOOL) return VINT(a[0].u.b ? 1 : 0);
	if (a[0].t == T_STR) return VINT(strtoll(a[0].u.s->p, NULL, 10));
	return mk_err("int: cannot convert");
}
static NlVal prim_float(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return mk_err("float expects 1 argument");
	if (a[0].t == T_FLOAT) return a[0];
	if (a[0].t == T_INT) return VFLOAT((double)a[0].u.i);
	if (a[0].t == T_STR) return VFLOAT(strtod(a[0].u.s->p, NULL));
	return mk_err("float: cannot convert");
}
static NlVal prim_str(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return mk_err("str expects 1 argument");
	if (a[0].t == T_STR) return a[0];
	return mk_str(inspect_v(a[0]));
}
static NlVal prim_copy(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return mk_err("copy expects 1 argument");
	if (a[0].t == T_MAP) return map_val(map_copy(a[0].u.m));
	if (a[0].t == T_LIST) {
		NlList *o = list_new();
		int i;
		for (i = 0; i < a[0].u.l->n; i++) list_push(o, a[0].u.l->p[i]);
		return list_val(o);
	}
	return a[0];
}
static NlVal prim_apply(NlVal *a, int n, NlEnv *env) {
	if (n != 2 || a[1].t != T_LIST) return mk_err("apply expects (fn, args_list)");
	return apply(a[0], a[1].u.l->p, a[1].u.l->n, env);
}
static NlVal prim_use(NlVal *a, int n, NlEnv *env) {
	if (n != 1) return mk_err("use expects 1 argument (path)");
	return use_module(str_raw(a[0]), env ? env : active);
}
static NlVal prim_must(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return mk_err("must expects 1 argument");
	if (nl_is_err(a[0])) {
		if (a[0].t == T_ERR) return a[0];
		return mk_err("%s", nl_format_err(a[0]));
	}
	return a[0];
}
static NlVal prim_is_err(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1) return VBOOL(0);
	return VBOOL(nl_is_err(a[0]));
}
static NlVal prim_builtins(NlVal *a, int n, NlEnv *env) {
	(void)a;
	(void)n;
	(void)env;
	return builtins_map();
}
static NlVal prim_vm_opcodes(NlVal *a, int n, NlEnv *env) {
	(void)a;
	(void)n;
	(void)env;
	return opcodes_map();
}
static NlVal prim_vm_run(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n < 1 || n > 2) return mk_err("vm_run expects (chunk, env?)");
	return nl_vm_run(a[0], n == 2 ? a[1] : VNULL);
}
static NlVal prim_parse_json(NlVal *a, int n, NlEnv *env) {
	(void)env;
	if (n != 1 || a[0].t != T_STR) return mk_err("argument to `parse_json` must be STRING");
	return json_parse(a[0].u.s->p, a[0].u.s->n);
}
static NlVal prim_json(NlVal *a, int n, NlEnv *env) {
	char *s;
	NlVal o;
	(void)env;
	if (n != 1) return mk_err("wrong number of arguments. got=%d, want=1", n);
	s = json_stringify(a[0]);
	o = mk_str(s);
	free(s);
	return o;
}
static NlVal prim_tool_call(NlVal *a, int n, NlEnv *env) {
	if (n != 2 || a[1].t != T_LIST) return mk_err("tool_call expects (name, args_list)");
	return call_tool(str_raw(a[0]), a[1].u.l->p, a[1].u.l->n, env);
}

static PrimEnt prims[] = {
	{"len", prim_len}, {"print", prim_print}, {"type", prim_type}, {"json", prim_json},
	{"parse_json", prim_parse_json}, {"range", prim_range}, {"keys", prim_keys}, {"values", prim_values},
	{"join", prim_join}, {"split", prim_split}, {"slice", prim_slice}, {"append", prim_append},
	{"ord", prim_ord}, {"chr", prim_chr}, {"is_digit", prim_is_digit}, {"is_alpha", prim_is_alpha},
	{"is_space", prim_is_space}, {"int", prim_int}, {"float", prim_float}, {"str", prim_str},
	{"copy", prim_copy}, {"apply", prim_apply}, {"tool_call", prim_tool_call}, {"use", prim_use},
	{"must", prim_must}, {"is_err", prim_is_err}, {"builtins", prim_builtins},
	{"vm_opcodes", prim_vm_opcodes}, {"vm_run", prim_vm_run}
};
static const int nprims = (int)(sizeof prims / sizeof prims[0]);

static int prim_id(const char *name) {
	int i;
	for (i = 0; i < nprims; i++)
		if (strcmp(prims[i].name, name) == 0) return i;
	return -1;
}
static NlVal prim_call(int id, NlVal *args, int n, NlEnv *env) { return prims[id].fn(args, n, env); }

static NlVal builtins_map(void) {
	NlMap *m = map_new();
	int i;
	for (i = 0; i < nprims; i++) {
		NlVal b = {.t = T_BUILTIN, .u.builtin = i};
		map_set(m, prims[i].name, b);
	}
	return map_val(m);
}
static NlVal opcodes_map(void) {
	NlMap *m = map_new();
	int i;
	for (i = 0; i < nop; i++) map_set(m, op_names[i], VINT(i));
	return map_val(m);
}

static NlVal call_tool(const char *name, NlVal *args, int n, NlEnv *env) {
	(void)env;
	if (strcmp(name, "fs.read") == 0) {
		int sz = 0;
		char *s;
		if (n != 1) return mk_err("!fs.read expects (path)");
		s = read_file(str_raw(args[0]), &sz);
		if (!s) return mk_err("fs.read error: %s", strerror(errno));
		{
			NlVal o = mk_str_n(s, sz);
			free(s);
			return o;
		}
	}
	if (strcmp(name, "fs.write") == 0) {
		const char *path, *data;
		int dn;
		char *tmp = NULL;
		if (n != 2) return mk_err("!fs.write expects (path, content)");
		path = str_raw(args[0]);
		if (args[1].t == T_STR) {
			data = args[1].u.s->p;
			dn = args[1].u.s->n;
		} else {
			tmp = json_stringify(args[1]);
			data = tmp;
			dn = (int)strlen(tmp);
		}
		if (!write_file(path, data, dn)) {
			free(tmp);
			return mk_err("fs.write error");
		}
		free(tmp);
		return VBOOL(1);
	}
	if (strcmp(name, "fs.list") == 0) {
		const char *path = n > 0 ? str_raw(args[0]) : ".";
		DIR *d = opendir(path);
		NlList *l;
		struct dirent *e;
		if (!d) return mk_err("fs.list error: %s", strerror(errno));
		l = list_new();
		while ((e = readdir(d))) {
			if (strcmp(e->d_name, ".") == 0 || strcmp(e->d_name, "..") == 0) continue;
			list_push(l, mk_str(e->d_name));
		}
		closedir(d);
		return list_val(l);
	}
	if (strcmp(name, "env.get") == 0) {
		const char *v;
		if (n != 1) return mk_err("!env.get expects (name)");
		v = getenv(str_raw(args[0]));
		return mk_str(v ? v : "");
	}
	if (strcmp(name, "time.now") == 0) return VINT((int64_t)time(NULL));
	if (strcmp(name, "time.sleep") == 0) {
		if (n != 1 || args[0].t != T_INT) return mk_err("!time.sleep expects INTEGER");
#ifdef _WIN32
		Sleep((DWORD)args[0].u.i);
#else
		usleep((useconds_t)args[0].u.i * 1000);
#endif
		return VNULL;
	}
	return mk_err("tool '%s' not registered or unknown", name);
}

static int ru16(unsigned char *c, int ip) { return (c[ip] << 8) | c[ip + 1]; }

typedef struct {
	NlVal *st;
	int sp, cap;
} Stk;

static void st_push(Stk *s, NlVal v) {
	if (s->sp >= s->cap) {
		s->cap = s->cap ? s->cap * 2 : 32;
		s->st = xrealloc(s->st, (size_t)s->cap * sizeof(NlVal));
	}
	s->st[s->sp++] = v;
}
static NlVal st_pop(Stk *s) { return s->st[--s->sp]; }
static NlVal st_peek(Stk *s) { return s->st[s->sp - 1]; }

static NlVal collect_iter(NlVal v, NlVal **out, int *n) {
	int i;
	if (v.t == T_LIST) {
		*out = v.u.l->p;
		*n = v.u.l->n;
		return VNULL;
	}
	if (v.t == T_STR) {
		int ncp = utf8_cp_len(v.u.s->p, v.u.s->n), i;
		NlVal *a = xmalloc((size_t)(ncp ? ncp : 1) * sizeof(NlVal));
		for (i = 0; i < ncp; i++) {
			int off = utf8_off(v.u.s->p, v.u.s->n, i);
			int end = utf8_off(v.u.s->p, v.u.s->n, i + 1);
			a[i] = mk_str_n(v.u.s->p + off, end - off);
		}
		*out = a;
		*n = ncp;
		return VNULL;
	}
	if (v.t == T_MAP) {
		char **ks = xmalloc((size_t)v.u.m->n * sizeof(char *));
		NlVal *a;
		for (i = 0; i < v.u.m->n; i++) ks[i] = v.u.m->k[i];
		qsort(ks, (size_t)v.u.m->n, sizeof(char *), cmpstr);
		a = xmalloc((size_t)(v.u.m->n ? v.u.m->n : 1) * sizeof(NlVal));
		for (i = 0; i < v.u.m->n; i++) a[i] = mk_str(ks[i]);
		free(ks);
		*out = a;
		*n = v.u.m->n;
		return VNULL;
	}
	return mk_err("cannot iterate over value");
}

static NlVal pipe_pred(NlVal left, NlVal pred, NlEnv *env, int filter) {
	NlVal *items = NULL;
	int n = 0, i, own = 0;
	NlList *out;
	if (left.t != T_LIST) {
		NlEnv *sc = env_enclosed(env);
		NlVal r;
		sc->dot = left;
		sc->has_dot = 1;
		r = apply(pred, NULL, 0, sc);
		if (nl_is_err(r) && r.t == T_ERR) return r;
		if (filter) return truthy(r) ? left : VNULL;
		return r;
	}
	items = left.u.l->p;
	n = left.u.l->n;
	(void)own;
	out = list_new();
	for (i = 0; i < n; i++) {
		NlEnv *sc = env_enclosed(env);
		NlVal r;
		sc->dot = items[i];
		sc->has_dot = 1;
		r = apply(pred, NULL, 0, sc);
		if (r.t == T_ERR) return r;
		if (filter) {
			if (truthy(r)) list_push(out, items[i]);
		} else
			list_push(out, r);
	}
	return list_val(out);
}

static const char *binop_name(int op) {
	switch (op) {
	case OP_ADD: return "+";
	case OP_SUB: return "-";
	case OP_MUL: return "*";
	case OP_DIV: return "/";
	case OP_MOD: return "%";
	case OP_EQ: return "==";
	case OP_NEQ: return "!=";
	case OP_LT: return "<";
	case OP_LTE: return "<=";
	case OP_GT: return ">";
	case OP_GTE: return ">=";
	case OP_IN: return "in";
	}
	return "?";
}

static NlVal apply(NlVal fn, NlVal *args, int n, NlEnv *env) {
	if (fn.t == T_FN) {
		NlEnv *ex;
		int i;
		NlVal r;
		if (apply_depth > 4096) return mk_err("call stack overflow");
		apply_depth++;
		ex = env_enclosed(fn.u.fn->env);
		if (env) {
			if (env->has_dot) {
				ex->dot = env->dot;
				ex->has_dot = 1;
			}
			if (!ex->file) {
				ex->file = env->file;
				ex->dir = env->dir;
			}
		}
		for (i = 0; i < fn.u.fn->nparams && i < n; i++) env_set(ex, fn.u.fn->params[i], args[i]);
		r = vm_exec(fn.u.fn, ex);
		apply_depth--;
		return r;
	}
	if (fn.t == T_BUILTIN) return prim_call(fn.u.builtin, args, n, env);
	return mk_err("not a function: %s", inspect_v(fn));
}

static NlVal lookup(NlEnv *env, const char *name) {
	NlVal v;
	int id;
	if (env_get(env, name, &v)) return v;
	id = prim_id(name);
	if (id >= 0) {
		NlVal b = {.t = T_BUILTIN, .u.builtin = id};
		return b;
	}
	if (strcmp(name, "load") == 0) {
		NlVal b = {.t = T_BUILTIN, .u.builtin = -2};
		return b;
	}
	if (strcmp(name, "use") == 0) {
		NlVal b = {.t = T_BUILTIN, .u.builtin = prim_id("use")};
		return b;
	}
	return mk_err("identifier not found: %s", name);
}

static NlVal vm_exec(NlFn *fn, NlEnv *env) {
	unsigned char *code = fn->code;
	int ncode = fn->ncode, ip = 0;
	Stk s = {0};
	while (ip < ncode) {
		int op = code[ip++];
		switch (op) {
		case OP_CONSTANT: {
			int idx = ru16(code, ip);
			ip += 2;
			st_push(&s, fn->consts[idx]);
			break;
		}
		case OP_TRUE: st_push(&s, VBOOL(1)); break;
		case OP_FALSE: st_push(&s, VBOOL(0)); break;
		case OP_NULL: st_push(&s, VNULL); break;
		case OP_POP: st_pop(&s); break;
		case OP_GET: {
			int idx = ru16(code, ip);
			NlVal v;
			ip += 2;
			v = lookup(env, fn->names[idx]);
			if (v.t == T_ERR) {
				return v;
			}
			st_push(&s, v);
			break;
		}
		case OP_SET: {
			int idx = ru16(code, ip);
			ip += 2;
			env_set(env, fn->names[idx], st_peek(&s));
			break;
		}
		case OP_ADD:
		case OP_SUB:
		case OP_MUL:
		case OP_DIV:
		case OP_MOD:
		case OP_EQ:
		case OP_NEQ:
		case OP_LT:
		case OP_LTE:
		case OP_GT:
		case OP_GTE:
		case OP_IN: {
			NlVal r = st_pop(&s), l = st_pop(&s), x = infix(binop_name(op), l, r);
			if (x.t == T_ERR) {
				return x;
			}
			st_push(&s, x);
			break;
		}
		case OP_MINUS: {
			NlVal x = prefix_minus(st_pop(&s));
			if (x.t == T_ERR) {
				return x;
			}
			st_push(&s, x);
			break;
		}
		case OP_BANG: st_push(&s, prefix_bang(st_pop(&s))); break;
		case OP_JUMP: ip = ru16(code, ip); break;
		case OP_JUMP_FALSE: {
			int t = ru16(code, ip);
			ip += 2;
			if (!truthy(st_peek(&s))) ip = t;
			break;
		}
		case OP_JUMP_TRUE: {
			int t = ru16(code, ip);
			ip += 2;
			if (truthy(st_peek(&s))) ip = t;
			break;
		}
		case OP_JUMP_OK: {
			int t = ru16(code, ip);
			NlVal top;
			ip += 2;
			top = st_peek(&s);
			if (top.t != T_NULL && !nl_is_err(top)) ip = t;
			break;
		}
		case OP_TO_BOOL: st_push(&s, VBOOL(truthy(st_pop(&s)))); break;
		case OP_ARRAY: {
			int n = ru16(code, ip), i;
			NlList *l = list_new();
			ip += 2;
			l->n = n;
			l->cap = n ? n : 1;
			l->p = xmalloc((size_t)l->cap * sizeof(NlVal));
			for (i = n - 1; i >= 0; i--) l->p[i] = st_pop(&s);
			st_push(&s, list_val(l));
			break;
		}
		case OP_MAP: {
			int n = ru16(code, ip), i;
			NlMap *m = map_new();
			ip += 2;
			for (i = 0; i < n; i++) {
				NlVal val = st_pop(&s), k = st_pop(&s);
				map_set(m, str_raw(k), val);
			}
			st_push(&s, map_val(m));
			break;
		}
		case OP_INDEX: {
			NlVal idx = st_pop(&s), left = st_pop(&s), x = index_get(left, idx);
			if (x.t == T_ERR) {
				return x;
			}
			st_push(&s, x);
			break;
		}
		case OP_SET_INDEX: {
			NlVal idx = st_pop(&s), obj = st_pop(&s), val = st_peek(&s);
			if (obj.t == T_MAP) map_set(obj.u.m, str_raw(idx), val);
			else if (obj.t == T_LIST && idx.t == T_INT && idx.u.i >= 0 && idx.u.i < obj.u.l->n)
				obj.u.l->p[idx.u.i] = val;
			break;
		}
		case OP_PROP: {
			int idx = ru16(code, ip);
			ip += 2;
			st_push(&s, prop_get(st_pop(&s), fn->names[idx]));
			break;
		}
		case OP_SET_PROP: {
			int idx = ru16(code, ip);
			NlVal obj, val;
			ip += 2;
			obj = st_pop(&s);
			val = st_peek(&s);
			if (obj.t == T_MAP) map_set(obj.u.m, fn->names[idx], val);
			break;
		}
		case OP_CALL: {
			int n = ru16(code, ip), i;
			NlVal *args = n ? xmalloc((size_t)n * sizeof(NlVal)) : NULL;
			NlVal f, r;
			ip += 2;
			for (i = n - 1; i >= 0; i--) args[i] = st_pop(&s);
			f = st_pop(&s);
			if (f.t == T_BUILTIN && f.u.builtin == -2) {
				if (n != 1) r = mk_err("load expects 1 argument (path)");
				else r = load_into(str_raw(args[0]), env);
			} else
				r = apply(f, args, n, env);
			free(args);
			if (r.t == T_ERR) {
				return r;
			}
			st_push(&s, r);
			break;
		}
		case OP_RETURN: {
			NlVal r = st_pop(&s);
			free(s.st);
			return r;
		}
		case OP_CLOSURE: {
			int idx = ru16(code, ip);
			NlFn *tmpl, *cl;
			ip += 2;
			tmpl = fn->consts[idx].u.fn;
			cl = fn_new();
			*cl = *tmpl;
			cl->env = env;
			st_push(&s, fn_val(cl));
			break;
		}
		case OP_DOT: {
			NlVal d = env_dot(env);
			if (!env->has_dot && !env_dot(env).t && d.t == T_NULL && !env->has_dot) {
				/* still may be in outer */
			}
			if (!env_get(env, ".", &d)) d = env_dot(env);
			if (!env->has_dot && env->outer && !env->has_dot) d = env_dot(env);
			d = env_dot(env);
			if (d.t == T_NULL && !env->has_dot && !(env->outer && env->outer->has_dot)) {
				int found = 0;
				NlEnv *e = env;
				while (e) {
					if (e->has_dot) {
						found = 1;
						d = e->dot;
						break;
					}
					e = e->outer;
				}
				if (!found) {
					return mk_err("context '.' is undefined outside pipeline or iteration");
				}
			}
			st_push(&s, d);
			break;
		}
		case OP_DOT_FIELD: {
			int idx = ru16(code, ip);
			NlVal d;
			char *field, *tok, *save = NULL;
			char buf[256];
			ip += 2;
			d = env_dot(env);
			{
				int found = 0;
				NlEnv *e = env;
				while (e) {
					if (e->has_dot) {
						found = 1;
						d = e->dot;
						break;
					}
					e = e->outer;
				}
				if (!found) {
					return mk_err("context '.' is undefined outside pipeline or iteration");
				}
			}
			field = fn->names[idx];
			snprintf(buf, sizeof buf, "%s", field);
			for (tok = strtok(buf, "."); tok; tok = strtok(NULL, ".")) d = prop_get(d, tok);
			st_push(&s, d);
			(void)save;
			break;
		}
		case OP_PIPE_FILTER:
		case OP_PIPE_MAP: {
			NlVal pred = st_pop(&s), left = st_pop(&s);
			NlVal r = pipe_pred(left, pred, env, op == OP_PIPE_FILTER);
			if (r.t == T_ERR) {
				return r;
			}
			st_push(&s, r);
			break;
		}
		case OP_PIPE_REDUCE: {
			NlVal f = st_pop(&s), left = st_pop(&s), acc;
			int i;
			if (left.t != T_LIST || left.u.l->n == 0) {
				st_push(&s, VNULL);
				break;
			}
			acc = left.u.l->p[0];
			for (i = 1; i < left.u.l->n; i++) {
				NlVal args[2] = {acc, left.u.l->p[i]};
				acc = apply(f, args, 2, env);
				if (acc.t == T_ERR) {
					return acc;
				}
			}
			st_push(&s, acc);
			break;
		}
		case OP_PIPE_CALL: {
			int n = ru16(code, ip), extra = n - 1, i;
			NlVal *extra_a = extra ? xmalloc((size_t)extra * sizeof(NlVal)) : NULL;
			NlVal f, left, r, *args;
			ip += 2;
			for (i = extra - 1; i >= 0; i--) extra_a[i] = st_pop(&s);
			f = st_pop(&s);
			left = st_pop(&s);
			args = xmalloc((size_t)n * sizeof(NlVal));
			args[0] = left;
			for (i = 0; i < extra; i++) args[i + 1] = extra_a[i];
			r = apply(f, args, n, env);
			free(extra_a);
			free(args);
			if (r.t == T_ERR) {
				return r;
			}
			st_push(&s, r);
			break;
		}
		case OP_TOOL: {
			int namei = ru16(code, ip), n, i;
			NlVal *args, r;
			ip += 2;
			n = ru16(code, ip);
			ip += 2;
			args = n ? xmalloc((size_t)n * sizeof(NlVal)) : NULL;
			for (i = n - 1; i >= 0; i--) args[i] = st_pop(&s);
			r = call_tool(fn->names[namei], args, n, env);
			free(args);
			if (r.t == T_ERR) {
				return r;
			}
			st_push(&s, r);
			break;
		}
		case OP_USE: {
			NlVal p = st_pop(&s);
			NlVal r = use_module(str_raw(p), env);
			if (r.t == T_ERR) {
				return r;
			}
			st_push(&s, r);
			break;
		}
		case OP_ITER: {
			NlVal sub = st_pop(&s), *items = NULL, err;
			int n = 0;
			NlIter *it;
			err = collect_iter(sub, &items, &n);
			if (err.t == T_ERR) {
				return err;
			}
			it = xmalloc(sizeof *it);
			if (sub.t == T_LIST) {
				it->p = xmalloc((size_t)(n ? n : 1) * sizeof(NlVal));
				if (n) memcpy(it->p, items, (size_t)n * sizeof(NlVal));
			} else
				it->p = items;
			it->n = n;
			it->i = 0;
			{
				NlVal v = {.t = T_ITER, .u.it = it};
				st_push(&s, v);
			}
			break;
		}
		case OP_ITER_NEXT: {
			int t = ru16(code, ip);
			NlVal top;
			ip += 2;
			top = st_peek(&s);
			if (top.t != T_ITER || top.u.it->i >= top.u.it->n) {
				st_pop(&s);
				ip = t;
				break;
			}
			st_push(&s, top.u.it->p[top.u.it->i++]);
			break;
		}
		case OP_SET_DOT:
			env->dot = st_peek(&s);
			env->has_dot = 1;
			break;
		default:
			return mk_err("unknown opcode %d", op);
		}
	}
	{
		NlVal r = s.sp ? s.st[s.sp - 1] : VNULL;
		free(s.st);
		return r;
	}
}

NlVal nl_vm_run(NlVal chunk, NlVal env_map) {
	NlFn *fn;
	NlEnv *env;
	NlVal r;
	NlVal cv = chunk_to_fn(chunk);
	if (cv.t != T_FN) return cv.t == T_ERR ? cv : mk_err("vm_run: chunk must be a map");
	fn = cv.u.fn;
	env = env_new();
	if (active) {
		env->file = active->file;
		env->dir = active->dir;
	}
	if (env_map.t == T_MAP) {
		int i;
		for (i = 0; i < env_map.u.m->n; i++) env_set(env, env_map.u.m->k[i], env_map.u.m->v[i]);
	}
	r = vm_exec(fn, env);
	if (env_map.t == T_MAP) {
		int i;
		for (i = 0; i < env->store->n; i++) map_set(env_map.u.m, env->store->k[i], env->store->v[i]);
	}
	return r;
}

static int cache_fresh(const char *std) {
	const char *names[] = {"lexer", "parser", "compile", "compiler"};
	time_t newest = 0, oldest = 0;
	int i;
	for (i = 0; i < 4; i++) {
		char *p = join_path(std, names[i]);
		char nl[1024], nlc[1024];
		time_t t;
		snprintf(nl, sizeof nl, "%s.nl", p);
		snprintf(nlc, sizeof nlc, "%s.nlc", p);
		free(p);
		if (!file_mtime(nl, &t)) return 0;
		if (t > newest) newest = t;
		if (!file_mtime(nlc, &t)) return 0;
		if (i == 0 || t < oldest) oldest = t;
	}
	return oldest >= newest;
}

static void write_std_cache(const char *std) {
	const char *names[] = {"lexer", "parser", "compile", "compiler"};
	int i;
	NlVal parse, compile;
	if (std_compiler.t != T_MAP) return;
	if (!map_get(std_compiler.u.m, "nl_parse", &parse) || !map_get(std_compiler.u.m, "nl_compile", &compile)) return;
	for (i = 0; i < 4; i++) {
		char *base = join_path(std, names[i]);
		char nl[1024];
		int n = 0;
		char *src;
		NlVal bc;
		snprintf(nl, sizeof nl, "%s.nl", base);
		free(base);
		src = read_file(nl, &n);
		if (!src) return;
		bc = compile_src(src);
		free(src);
		if (nl_is_err(bc)) return;
		{
			char *js = json_stringify(bc);
			char *np = nlc_path(nl);
			write_file(np, js, (int)strlen(js));
			free(js);
			free(np);
		}
	}
}

static int boot_from_cache(const char *std) {
	const char *parts[] = {"lexer.nlc", "parser.nlc", "compile.nlc"};
	NlVal L, P, K, comp, env, C;
	int i;
	NlEnv *host = env_new();
	host->file = join_path(std, "compiler.nl");
	host->dir = xstrdup(std);
	active = host;
	{
		NlVal vs[3];
		for (i = 0; i < 3; i++) {
			char *p = join_path(std, parts[i]);
			NlVal bc = read_nlc(p);
			free(p);
			if (bc.t == T_NULL) return 0;
			vs[i] = nl_vm_run(bc, map_val(map_new()));
			if (nl_is_err(vs[i])) return 0;
		}
		L = vs[0];
		P = vs[1];
		K = vs[2];
	}
	env = map_val(map_new());
	map_set(env.u.m, "L", L);
	map_set(env.u.m, "P", P);
	map_set(env.u.m, "K", K);
	{
		char *p = join_path(std, "compiler.nlc");
		comp = read_nlc(p);
		free(p);
	}
	if (comp.t == T_NULL) return 0;
	C = nl_vm_run(comp, env);
	if (nl_is_err(C) || C.t != T_MAP) return 0;
	if (!map_has(C.u.m, "nl_eval") || !map_has(C.u.m, "nl_compile")) return 0;
	std_compiler = C;
	booted = 1;
	env_set(host, "C", C);
	return 1;
}

const char *nl_find_std(void) {
	char cwd[4096];
	const char *env = getenv("NL_STD");
	if (env && exists_file(join_path(env, "compiler.nlc"))) {
		snprintf(std_found, sizeof std_found, "%s", env);
		return std_found;
	}
	getcwd(cwd, sizeof cwd);
	{
		char *d = xstrdup(cwd);
		for (;;) {
			char *p = join_path(d, "std/compiler.nlc");
			if (exists_file(p)) {
				char *s = join_path(d, "std");
				snprintf(std_found, sizeof std_found, "%s", s);
				free(s);
				free(p);
				free(d);
				return std_found;
			}
			free(p);
			{
				char *parent = dir_of(d);
				if (strcmp(parent, d) == 0) {
					free(parent);
					break;
				}
				free(d);
				d = parent;
			}
		}
		free(d);
	}
#ifdef _WIN32
	{
		char exe[MAX_PATH];
		GetModuleFileNameA(NULL, exe, MAX_PATH);
		{
			char *d = dir_of(exe);
			char *p = join_path(d, "std/compiler.nlc");
			if (exists_file(p)) {
				char *s = join_path(d, "std");
				snprintf(std_found, sizeof std_found, "%s", s);
				free(s);
				free(p);
				free(d);
				return std_found;
			}
			free(p);
			free(d);
		}
	}
#endif
	return NULL;
}

int nl_boot(const char *std_dir) {
	const char *std = std_dir ? std_dir : nl_find_std();
	if (!std) return 0;
	if (!exists_file(join_path(std, "compiler.nlc"))) return 0;
	if (!boot_from_cache(std)) return 0;
	if (!cache_fresh(std)) {
		write_std_cache(std);
		if (cache_fresh(std)) {
			booted = 0;
			std_compiler = VNULL;
			if (!boot_from_cache(std)) return 1;
		}
	}
	return 1;
}

NlVal nl_eval(const char *src, const char *file) {
	NlVal fn, env, a[2], r;
	NlEnv *host;
	if (!booted && !nl_boot(NULL)) return mk_err("failed to boot from std/*.nlc");
	if (!map_get(std_compiler.u.m, "nl_eval", &fn)) return mk_err("compiler is missing nl_eval");
	host = env_new();
	if (file) {
		host->file = abspath(file);
		host->dir = dir_of(host->file);
	} else if (active) {
		host->file = active->file;
		host->dir = active->dir;
	}
	active = host;
	env = VNULL;
	a[0] = mk_str(src);
	a[1] = env;
	r = apply(fn, a, 2, host);
	return r;
}

void nl_init(void) { /* reserved */ }
