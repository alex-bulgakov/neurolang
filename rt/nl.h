#ifndef NL_H
#define NL_H

#include <stdint.h>

typedef enum {
	T_NULL, T_BOOL, T_INT, T_FLOAT, T_STR, T_LIST, T_MAP, T_FN, T_BUILTIN, T_ERR, T_ITER
} NlType;

typedef struct NlStr NlStr;
typedef struct NlList NlList;
typedef struct NlMap NlMap;
typedef struct NlFn NlFn;
typedef struct NlIter NlIter;
typedef struct NlEnv NlEnv;
typedef struct NlVal NlVal;

struct NlVal {
	NlType t;
	union {
		int b;
		int64_t i;
		double f;
		NlStr *s;
		NlList *l;
		NlMap *m;
		NlFn *fn;
		int builtin;
		char *err;
		NlIter *it;
	} u;
};

void nl_init(void);
int nl_boot(const char *std_dir);
NlVal nl_eval(const char *src, const char *file);
NlVal nl_vm_run(NlVal chunk, NlVal env_map);
const char *nl_inspect(NlVal v);
const char *nl_format_err(NlVal v);
int nl_is_err(NlVal v);
const char *nl_find_std(void);

#endif
