# SPDX-License-Identifier: (GPL-2.0 OR BSD-2-Clause)

XDP_DEPS := llb_kern_packet.c  \
            llb_kern_parse.c   \
            llb_kern_l2.c      \
            llb_kern_l3.c      \
            llb_kern_sess.c    \
            llb_kern_policer.c

XDP_TARGETS  := llb_xdp_main
TC_TARGETS   := llb_ebpf_main
USER_TARGETS := loxilb_libdp

LLC ?= llc
CLANG ?= clang
CC := gcc

LIBBPF_DIR = ../libbpf/src/
COMMON_DIR = ../common

LDFLAGS +=  -lz 

include $(COMMON_DIR)/common.mk
