# SPDX-License-Identifier: (GPL-2.0)
CC := gcc

all: common_user_bpf_xdp.o common_libbpf.o common_pdi.o

CFLAGS := -g -Wall -I../libbpf/src/

LIBBPF_DIR = ../libbpf/src/
CFLAGS += -I$(LIBBPF_DIR)/build/usr/include/  -I../headers

common_user_bpf_xdp.o: common_user_bpf_xdp.c common_user_bpf_xdp.h common_params.h
	$(CC) $(CFLAGS) -c -o $@ $<

common_libbpf.o: common_libbpf.c common_libbpf.h
	$(CC) $(CFLAGS) -c -o $@ $<

.PHONY: clean

clean:
	rm -f *.o
