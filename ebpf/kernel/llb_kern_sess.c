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
dp_do_sess4_lkup(void *ctx, struct xfi *F)
{
  struct dp_sess4_key key;
  struct dp_sess_tact *act;

  if (F->tm.tunnel_id && F->tm.tun_type != LLB_TUN_GTP) {
    return 0;
  }

  key.r = 0;
  if (F->tm.tunnel_id) {
    key.daddr = F->il3m.ip.daddr;
    key.saddr = F->il3m.ip.saddr;
    key.teid = bpf_ntohl(F->tm.tunnel_id);
  } else {
    key.daddr = F->l3m.ip.daddr;
    key.saddr = F->l3m.ip.saddr;
    key.teid = 0;
  }

  LL_DBG_PRINTK("[SESS4] -- Lookup\n");
  LL_DBG_PRINTK("[SESS4] daddr %x\n", key.daddr);
  LL_DBG_PRINTK("[SESS4] saddr %d\n", key.saddr);
  LL_DBG_PRINTK("[SESS4] teid 0x%x\n", key.teid);

  F->pm.table_id = LL_DP_SESS4_MAP;

  act = bpf_map_lookup_elem(&sess_v4_map, &key);
  if (!act) {
    LL_DBG_PRINTK("[SESS4] miss");
    goto drop;
  }

  F->pm.phit |= LLB_DP_SESS_HIT;
  dp_do_map_stats(ctx, F, LL_DP_SESS4_STATS_MAP, act->ca.cidx);

  if (act->ca.act_type == DP_SET_DROP) {
    goto drop;
  } else if (act->ca.act_type == DP_SET_RM_GTP) {
    dp_pipe_set_rm_gtp_tun(ctx, F);
    F->qm.qfi = act->qfi;
  } else {
    F->tm.new_tunnel_id = act->teid;
    F->tm.tun_type = LLB_TUN_GTP;
    F->qm.qfi = act->qfi;
    F->tm.tun_rip = act->rip;
    F->tm.tun_sip = act->sip;
  }

  return 0;

drop:
  LLBS_PPLN_DROP(F);
  return 0;
}
