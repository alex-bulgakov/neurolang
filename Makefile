CC ?= gcc
CFLAGS ?= -std=c11 -O2
neurolang: rt/nl.c rt/main.c rt/nl.h
	$(CC) $(CFLAGS) -o $@ rt/nl.c rt/main.c
clean:
	rm -f neurolang neurolang.exe
.PHONY: clean
