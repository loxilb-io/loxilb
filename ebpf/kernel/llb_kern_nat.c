/*
 *  llb_kern_nat.c: LoxiLB Kernel eBPF Stateful NAT Processing
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int __always_inline
dp_sel_nat_ep(void *ctx, struct dp_natv4_tacts *act)
{
  int sel = -1;
  uint8_t n = 0;
  uint16_t i = 0;
  struct mf_xfrm_inf *nxfrm_act;

  if (act->sel_type == NAT_LB_SEL_RR) {
    bpf_spin_lock(&act->lock);
    i = act->sel_hint; 

    while (n < LLB_MAX_NXFRMS) {
      if (i >= 0 && i < LLB_MAX_NXFRMS) {
        nxfrm_act = &act->nxfrms[i];
        if (nxfrm_act < act + 1) {
          if (nxfrm_act->inactive == 0) { 
            act->sel_hint = (i + 1) % act->nxfrm;
            sel = i;
            break;
          }
        }
      }
      i++;
      i = i % act->nxfrm;
      n++;
    }
    bpf_spin_unlock(&act->lock);
  } else if (act->sel_type == NAT_LB_SEL_HASH) {
    sel = dp_get_pkt_hash(ctx) % act->nxfrm;
    if (sel >= 0 && sel < LLB_MAX_NXFRMS) {
      /* Fall back if hash selection gives us a deadend */
      if (act->nxfrms[sel].inactive) {
        for (i = 0; i < LLB_MAX_NXFRMS; i++) {
          if (act->nxfrms[i].inactive == 0) {
            sel = i;
            break;
          }
        }
      }
    }
  }

  return sel;
}

static int __always_inline
dp_do_nat4_rule_lkup(void *ctx, struct xfi *F)
{
  struct dp_natv4_key *key = (void *)F->km.skey;
  struct mf_xfrm_inf *nxfrm_act;
  struct dp_natv4_tacts *act;
  __u32 sel;

  key->daddr = F->l3m.ip.daddr;
  if (F->l3m.nw_proto != IPPROTO_ICMP) {
    key->dport = F->l3m.dest;
  } else {
    key->dport = 0;
  }
  key->zone = F->pm.zone;
  key->l4proto = F->l3m.nw_proto;

  LL_DBG_PRINTK("[NAT4] --Lookup\n");

  F->pm.table_id = LL_DP_NAT4_MAP;

  act = bpf_map_lookup_elem(&nat_v4_map, key);
  if (!act) {
    /* Default action - Nothing to do */
    F->pm.nf &= ~LLB_NAT_SRC;
    return 0;
  }

  LL_DBG_PRINTK("[NAT4] action %d pipe %x\n",
                 act->ca.act_type, F->pm.pipe_act);

  if (act->ca.act_type == DP_SET_SNAT || 
      act->ca.act_type == DP_SET_DNAT) {
    sel = dp_sel_nat_ep(ctx, act);

    bpf_printk("lb-sel %d", sel);

    /* FIXME - Do not select inactive end-points 
     * Need multi-passes for selection
     */
    if (sel >= 0 && sel < LLB_MAX_NXFRMS) {
      nxfrm_act = &act->nxfrms[sel];

      if (nxfrm_act < act + 1) {
        F->pm.nf = act->ca.act_type == DP_SET_SNAT ? LLB_NAT_SRC : LLB_NAT_DST;
        F->l4m.nxip = nxfrm_act->nat_xip;
        F->l4m.nxport = nxfrm_act->nat_xport;
        F->l4m.sel_aid = sel;
        F->pm.rule_id =  act->ca.cidx;
        LL_DBG_PRINTK("[NAT4] ACT %x\n", F->pm.nf);
        /* Special case related to host-dnat */
        if (F->l3m.ip.saddr == F->l4m.nxip && F->pm.nf == LLB_NAT_DST) {
          F->l4m.nxip = 0;
        }
      }
    }
  } else { 
    LLBS_PPLN_DROP(F);
  }

  return 0;
}
