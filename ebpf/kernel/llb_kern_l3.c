/*
 *  llb_kern_l3.c: LoxiLB Kernel eBPF L3 Processing Implementation
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int __always_inline
dp_do_rtv4_fwd(void *ctx, struct xfi *F)
{
  struct iphdr *iph = DP_TC_PTR(DP_PDATA(ctx) + F->pm.l3_off);
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  if (iph + 1 > dend)  {
    LLBS_PPLN_DROP(F);
    return -1;
  }
  ip_decrease_ttl(iph);
  return 0;
}

static int __always_inline
dp_pipe_set_l32_tun_nh(void *ctx, struct xfi *F,
                       struct dp_rt_nh_act *rnh)
{
  struct dp_rt_l2nh_act *nl2;
  F->pm.nh_num = rnh->nh_num;
  /*
   * We do not set out_bd here. After NH lookup match is
   * found and packet tunnel insertion is done, BD is set accordingly
   */
  /*F->pm.bd = rnh->bd;*/
  F->tm.new_tunnel_id = rnh->tid;

  nl2 = &rnh->l2nh;
  memcpy(F->l2m.dl_dst, nl2->dmac, 6);
  memcpy(F->l2m.dl_src, nl2->smac, 6);
  memcpy(F->pm.lkup_dmac, nl2->dmac, 6);
  F->pm.bd = nl2->bd;

  LL_DBG_PRINTK("[RTFW] new-vx nh %u\n", F->pm.nh_num);
  return 0;
}

static int __always_inline
dp_do_rtv4_lkup(void *ctx, struct xfi *F, void *fa_)
{
  //struct dp_rtv4_key key = { 0 };
  struct dp_rtv4_key *key = (void *)F->km.skey;
  struct dp_rt_tact *act;
#ifdef HAVE_DP_FC
  struct dp_fc_tacts *fa = fa_;
#endif

  key->l.prefixlen = 48; /* 16-bit zone + 32-bit prefix */
  key->v4k[0] = F->pm.zone >> 8 & 0xff;
  key->v4k[1] = F->pm.zone & 0xff;

  if (F->pm.nf & LLB_NAT_DST) {
    *(__u32 *)&key->v4k[2] = F->l4m.nxip?:F->l3m.ip.saddr;
  } else {
    if (F->pm.nf & LLB_NAT_SRC && F->l4m.nxip == 0) {
      *(__u32 *)&key->v4k[2] = F->l3m.ip.saddr;
    } else {
      if (F->tm.new_tunnel_id && F->tm.tun_type == LLB_TUN_GTP) {
        /* In case of GTP, there is no interface created in OS 
         * which has a specific route through it. So, this hack !!
         */
        *(__u32 *)&key->v4k[2] = F->tm.tun_rip;
      } else {
        *(__u32 *)&key->v4k[2] = F->l3m.ip.daddr;
      }
    }
  }
  
  LL_DBG_PRINTK("[RTFW] --Lookup\n");
  LL_DBG_PRINTK("[RTFW] Zone %d 0x%x\n",
                 F->pm.zone, *(__u32 *)&key->v4k[2]);

  F->pm.table_id = LL_DP_RTV4_MAP;

  act = bpf_map_lookup_elem(&rt_v4_map, key);
  if (!act) {
    /* Default action - Nothing to do */
    F->pm.nf &= ~LLB_NAT_SRC;
    return 0;
  }

  F->pm.phit |= LLB_XDP_RT_HIT;
  dp_do_map_stats(ctx, F, LL_DP_RTV4_STATS_MAP, act->ca.cidx);

  LL_DBG_PRINTK("[RTFW] action %d pipe %x\n",
                 act->ca.act_type, F->pm.pipe_act);

  if (act->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(F);
  } else if (act->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(F);
  } else if (act->ca.act_type == DP_SET_RDR_PORT) {
    struct dp_rdr_act *ra = &act->port_act;
    LLBS_PPLN_RDR(F);
    F->pm.oport = ra->oport;
  } else if (act->ca.act_type == DP_SET_RT_NHNUM) {
    struct dp_rt_nh_act *rnh = &act->rt_nh;
    F->pm.nh_num = rnh->nh_num;
    return dp_do_rtv4_fwd(ctx, F);
  } /*else if (act->ca.act_type == DP_SET_L3RT_TUN_NH) {
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[DP_SET_L3RT_TUN_NH];
    ta->ca.act_type = DP_SET_L3RT_TUN_NH;
    memcpy(&ta->nh_act,  &act->rt_nh, sizeof(act->rt_nh));
#endif
    return dp_pipe_set_l32_tun_nh(ctx, F, &act->rt_nh);
  } */ else {
    LLBS_PPLN_DROP(F);
  }

  return 0;
}

static int __always_inline
dp_pipe_set_nat(void *ctx, struct xfi *F, 
                struct dp_nat_act *na, int do_snat)
{
  F->pm.nf = do_snat ? LLB_NAT_SRC : LLB_NAT_DST;
  F->l4m.nxip = na->xip;
  F->l4m.nxport = na->xport;
  LL_DBG_PRINTK("[ACL4] NAT ACT %x\n", F->pm.nf);

  return 0;
}

static int __always_inline
dp_do_aclv4_lkup(void *ctx, struct xfi *F, void *fa_)
{
  struct dp_ctv4_key key;
  struct dp_aclv4_tact *act;
#ifdef HAVE_DP_FC
  struct dp_fc_tacts *fa = fa_;
#endif

  key.daddr = F->l3m.ip.daddr;
  key.saddr = F->l3m.ip.saddr;
  key.sport = F->l3m.source;
  key.dport = F->l3m.dest;
  key.l4proto = F->l3m.nw_proto;
  key.zone = F->pm.zone;
  key.r = 0;

  LL_DBG_PRINTK("[ACL4] -- Lookup\n");
  LL_DBG_PRINTK("[ACL4] key-sz %d\n", sizeof(key));
  LL_DBG_PRINTK("[ACL4] daddr %x\n", key.daddr);
  LL_DBG_PRINTK("[ACL4] saddr %d\n", key.saddr);
  LL_DBG_PRINTK("[ACL4] sport %d\n", key.sport);
  LL_DBG_PRINTK("[ACL4] dport %d\n", key.dport);
  LL_DBG_PRINTK("[ACL4] l4proto %d\n", key.l4proto);

  F->pm.table_id = LL_DP_ACLV4_MAP;

  act = bpf_map_lookup_elem(&acl_v4_map, &key);
  if (!act) {
    LL_DBG_PRINTK("[ACL4] miss");
    goto ct_trk;
  }

  F->pm.phit |= LLB_DP_ACL_HIT;
  act->lts = bpf_ktime_get_ns();

#ifdef HAVE_DP_FC
  fa->ca.cidx = act->ca.cidx;
#endif

  dp_do_map_stats(ctx, F, LL_DP_ACLV4_STATS_MAP, act->ca.cidx);

  if (act->ca.act_type == DP_SET_DO_CT) {
    goto ct_trk;
  } else if (act->ca.act_type == DP_SET_NOP) {
    struct dp_rdr_act *ar = &act->port_act;
    if (F->pm.tcp_flags & (LLB_TCP_FIN|LLB_TCP_RST)) {
      ar->fr = 1;
    }

    if (ar->fr == 1) {
      goto ct_trk;
    }

    return 0;
  } else if (act->ca.act_type == DP_SET_RDR_PORT) {
    struct dp_rdr_act *ar = &act->port_act;

    if (F->pm.tcp_flags & (LLB_TCP_FIN|LLB_TCP_RST)) {
      ar->fr = 1;
    }

    if (ar->fr == 1) {
      goto ct_trk;
    }

    LLBS_PPLN_RDR_PRIO(F);
    F->pm.oport = ar->oport;
    return 0;
  } else if (act->ca.act_type == DP_SET_SNAT || 
             act->ca.act_type == DP_SET_DNAT) {
    struct dp_nat_act *na;
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[
                                  act->ca.act_type == DP_SET_SNAT ?
                                  DP_SET_SNAT : DP_SET_DNAT];
    ta->ca.act_type = act->ca.act_type;
    memcpy(&ta->nat_act,  &act->nat_act, sizeof(act->nat_act));
#endif

    na = &act->nat_act;

    if (F->pm.tcp_flags & (LLB_TCP_FIN|LLB_TCP_RST)) {
      na->fr = 1;
    }

    dp_pipe_set_nat(ctx, F, na,
                    act->ca.act_type == DP_SET_SNAT ? 1: 0);


    if (na->fr == 1 || na->doct) {
      goto ct_trk;
    }
    return 0;
  }

  if (act->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(F);
  } else if (act->ca.act_type == DP_SET_TOCP) {
    /*LLBS_PPLN_TRAP(F);*/
    LLBS_PPLN_TRAPC(F, LLB_PIPE_RC_ACL_MISS);
  } else if (act->ca.act_type == DP_SET_SESS_FWD_ACT) {
    struct dp_sess_act *pa = &act->pdr_sess_act; 
    F->pm.sess_id = pa->sess_id;
    return 0;
  } else {
    LLBS_PPLN_DROP(F);
  }

  return 0;

ct_trk:
  return dp_tail_call(ctx, F, fa_, LLB_DP_CT_PGM_ID);
}

static void __always_inline
dp_do_ipv4_fwd(void *ctx,  struct xfi *F, void *fa_)
{
  if (F->pm.phit & LLB_DP_TMAC_HIT) {

    /* If some pipeline block already set a redirect before this,
     * we honor this and dont do further l3 processing 
     */
    if ((F->pm.pipe_act & LLB_PIPE_RDR_MASK) == 0) {
      dp_do_rtv4_lkup(ctx, F, fa_);
    }
  }
}

static int __always_inline
dp_ing_ipv4(void *ctx,  struct xfi *F, void *fa_)
{
  if (F->pm.upp) dp_do_sess4_lkup(ctx, F);
  dp_do_aclv4_lkup(ctx, F, fa_);
  dp_do_ipv4_fwd(ctx, F, fa_);

  return 0;
}
