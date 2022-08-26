/*
 * llb_ebpf_main.c: LoxiLB TC eBPF Main processing
 * Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
#include <linux/bpf.h>
#include <linux/in.h>
#include <linux/if_arp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define HAVE_DP_FC 1

#include "llb_kern_entry.c"

char _license[] SEC("license") = "GPL";
