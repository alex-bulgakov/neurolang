#include "nl.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static char *read_all(const char *path) {
	FILE *f = fopen(path, "rb");
	long n;
	char *s;
	if (!f) return NULL;
	fseek(f, 0, SEEK_END);
	n = ftell(f);
	fseek(f, 0, SEEK_SET);
	s = malloc((size_t)n + 1);
	if (!s) {
		fclose(f);
		return NULL;
	}
	if (n) fread(s, 1, (size_t)n, f);
	s[n] = 0;
	fclose(f);
	return s;
}

static void die_val(NlVal v) {
	fprintf(stderr, "%s\n", nl_format_err(v));
	exit(1);
}

static void run_src(const char *src, const char *file) {
	NlVal r;
	if (!nl_boot(NULL)) {
		fprintf(stderr, "failed to boot from std/*.nlc\n");
		exit(1);
	}
	r = nl_eval(src, file);
	if (nl_is_err(r)) die_val(r);
}

static void run_file(const char *path) {
	char *s = read_all(path);
	if (!s) {
		fprintf(stderr, "Error reading file %s\n", path);
		exit(1);
	}
	run_src(s, path);
	free(s);
}

static void cmd_eval(const char *code) {
	NlVal r;
	if (!nl_boot(NULL)) {
		fprintf(stderr, "failed to boot from std/*.nlc\n");
		exit(1);
	}
	r = nl_eval(code, NULL);
	if (nl_is_err(r)) die_val(r);
	if (r.t != T_NULL) printf("%s\n", nl_inspect(r));
}

static void cmd_check(void) {
	char *ok, *bad;
	NlVal r, e;
	if (!nl_boot(NULL)) {
		fprintf(stderr, "failed to boot from std/*.nlc\n");
		exit(1);
	}
	ok = read_all("tests/all.nl");
	if (!ok) {
		fprintf(stderr, "tests/all.nl not found\n");
		exit(1);
	}
	r = nl_eval(ok, "tests/all.nl");
	free(ok);
	if (nl_is_err(r)) die_val(r);
	bad = read_all("tests/err.nl");
	if (!bad) {
		fprintf(stderr, "tests/err.nl not found\n");
		exit(1);
	}
	e = nl_eval(bad, "tests/err.nl");
	free(bad);
	if (!nl_is_err(e)) {
		fprintf(stderr, "tests/err.nl should fail\n");
		exit(1);
	}
	printf("\"ok\"\n");
}

int main(int argc, char **argv) {
	const char *cmd;
	nl_init();
	if (argc < 2) {
		fprintf(stderr, "NeuroLang native (no Go)\nUsage: neurolang run <file.nl> | eval <code> | check | version\n");
		return 0;
	}
	cmd = argv[1];
	if (strcmp(cmd, "run") == 0) {
		if (argc < 3) {
			fprintf(stderr, "Usage: neurolang run <file.nl>\n");
			return 1;
		}
		run_file(argv[2]);
		return 0;
	}
	if (strcmp(cmd, "eval") == 0) {
		if (argc < 3) {
			fprintf(stderr, "Usage: neurolang eval \"<code>\"\n");
			return 1;
		}
		cmd_eval(argv[2]);
		return 0;
	}
	if (strcmp(cmd, "check") == 0) {
		cmd_check();
		return 0;
	}
	if (strcmp(cmd, "version") == 0 || strcmp(cmd, "-v") == 0) {
		printf("NeuroLang v0.19.0 (native C runtime)\n");
		return 0;
	}
	if (strlen(cmd) > 3 && strcmp(cmd + strlen(cmd) - 3, ".nl") == 0) {
		run_file(cmd);
		return 0;
	}
	fprintf(stderr, "Unknown command: %s\n", cmd);
	return 1;
}
