/*
 *  llb_kern_sess.c: LoxiLB kernel eBPF Subscriber Session Implementation
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0 
 */
#include <linux/bpf.h>
#include <linux/in.h>
#include <linux/if_arp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include "../common/parsing_helpers.h"

static int __always_inline
dp_pipe_set_rm_gtp_tun(void *ctx, struct xfi *F)
{
  LL_DBG_PRINTK("[SESS] rm-gtp \n");
  return dp_pop_outer_metadata(ctx, F, 0);
}

static int __always_inline
dp_ing_session(struct xdp_md *ctx,  struct xfi *F)
{
  LL_DBG_PRINTK("[ING] SESS--\n");

  return 0;
}
