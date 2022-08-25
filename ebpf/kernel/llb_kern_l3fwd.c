/*
 *  llb_kern_l3fwd.c: LoxiLB Kernel eBPF L3 forwarder Implementation
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int __always_inline
dp_do_rtv4_fwd(void *ctx, struct xfi *xf)
{
  struct iphdr *iph = DP_TC_PTR(DP_PDATA(ctx) + xf->pm.l3_off);
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  if (iph + 1 > dend)  {
    LLBS_PPLN_DROP(xf);
    return -1;
  }
  ip_decrease_ttl(iph);
  return 0;
}

static int __always_inline
dp_pipe_set_l32_tun_nh(void *ctx, struct xfi *xf,
                       struct dp_rt_nh_act *rnh)
{
  struct dp_rt_l2nh_act *nl2;
  xf->pm.nh_num = rnh->nh_num;
  /*
   * We do not set out_bd here. After NH lookup match is
   * found and packet tunnel insertion is done, BD is set accordingly
   */
  /*xf->pm.bd = rnh->bd;*/
  xf->tm.new_tunnel_id = rnh->tid;

  nl2 = &rnh->l2nh;
  memcpy(xf->l2m.dl_dst, nl2->dmac, 6);
  memcpy(xf->l2m.dl_src, nl2->smac, 6);
  memcpy(xf->pm.lkup_dmac, nl2->dmac, 6);
  xf->pm.bd = nl2->bd;

  LL_DBG_PRINTK("[RTFW] new-vx nh %u\n", xf->pm.nh_num);
  return 0;
}

static int __always_inline
dp_do_rtv4_lkup(void *ctx, struct xfi *xf, void *fa_)
{
  //struct dp_rtv4_key key = { 0 };
  struct dp_rtv4_key *key = (void *)xf->km.skey;
  struct dp_rt_tact *act;

  key->l.prefixlen = 48; /* 16-bit zone + 32-bit prefix */
  key->v4k[0] = xf->pm.zone >> 8 & 0xff;
  key->v4k[1] = xf->pm.zone & 0xff;

  if (xf->pm.nf & LLB_NAT_DST) {
    *(__u32 *)&key->v4k[2] = xf->l4m.nxip?:xf->l3m.ip.saddr;
  } else {
    if (xf->pm.nf & LLB_NAT_SRC && xf->l4m.nxip == 0) {
      *(__u32 *)&key->v4k[2] = xf->l3m.ip.saddr;
    } else {
      if (xf->tm.new_tunnel_id && xf->tm.tun_type == LLB_TUN_GTP) {
        /* In case of GTP, there is no interface created in OS 
         * which has a specific route through it. So, this hack !!
         */
        *(__u32 *)&key->v4k[2] = xf->tm.tun_rip;
      } else {
        *(__u32 *)&key->v4k[2] = xf->l3m.ip.daddr;
      }
    }
  }
  
  LL_DBG_PRINTK("[RTFW] --Lookup\n");
  LL_DBG_PRINTK("[RTFW] Zone %d 0x%x\n",
                 xf->pm.zone, *(__u32 *)&key->v4k[2]);

  xf->pm.table_id = LL_DP_RTV4_MAP;

  act = bpf_map_lookup_elem(&rt_v4_map, key);
  if (!act) {
    /* Default action - Nothing to do */
    xf->pm.nf &= ~LLB_NAT_SRC;
    return 0;
  }

  xf->pm.phit |= LLB_XDP_RT_HIT;
  dp_do_map_stats(ctx, xf, LL_DP_RTV4_STATS_MAP, act->ca.cidx);

  LL_DBG_PRINTK("[RTFW] action %d pipe %x\n",
                 act->ca.act_type, xf->pm.pipe_act);

  if (act->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(xf);
  } else if (act->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(xf);
  } else if (act->ca.act_type == DP_SET_RDR_PORT) {
    struct dp_rdr_act *ra = &act->port_act;
    LLBS_PPLN_RDR(xf);
    xf->pm.oport = ra->oport;
  } else if (act->ca.act_type == DP_SET_RT_NHNUM) {
    struct dp_rt_nh_act *rnh = &act->rt_nh;
    xf->pm.nh_num = rnh->nh_num;
    return dp_do_rtv4_fwd(ctx, xf);
  } /*else if (act->ca.act_type == DP_SET_L3RT_TUN_NH) {
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[DP_SET_L3RT_TUN_NH];
    ta->ca.act_type = DP_SET_L3RT_TUN_NH;
    memcpy(&ta->nh_act,  &act->rt_nh, sizeof(act->rt_nh));
#endif
    return dp_pipe_set_l32_tun_nh(ctx, xf, &act->rt_nh);
  } */ else {
    LLBS_PPLN_DROP(xf);
  }

  return 0;
}

static int __always_inline
dp_pipe_set_nat(void *ctx, struct xfi *xf, 
                struct dp_nat_act *na, int do_snat)
{
  xf->pm.nf = do_snat ? LLB_NAT_SRC : LLB_NAT_DST;
  xf->l4m.nxip = na->xip;
  xf->l4m.nxport = na->xport;
  LL_DBG_PRINTK("[ACL4] NAT ACT %x\n", xf->pm.nf);

  return 0;
}

static int __always_inline
dp_do_aclv4_lkup(void *ctx, struct xfi *xf, void *fa_)
{
  struct dp_ctv4_key key;
  struct dp_aclv4_tact *act;
#ifdef HAVE_DP_FC
  struct dp_fc_tacts *fa = fa_;
#endif

  key.daddr = xf->l3m.ip.daddr;
  key.saddr = xf->l3m.ip.saddr;
  key.sport = xf->l3m.source;
  key.dport = xf->l3m.dest;
  key.l4proto = xf->l3m.nw_proto;
  key.zone = xf->pm.zone;
  key.r = 0;

  LL_DBG_PRINTK("[ACL4] -- Lookup\n");
  LL_DBG_PRINTK("[ACL4] key-sz %d\n", sizeof(key));
  LL_DBG_PRINTK("[ACL4] daddr %x\n", key.daddr);
  LL_DBG_PRINTK("[ACL4] saddr %d\n", key.saddr);
  LL_DBG_PRINTK("[ACL4] sport %d\n", key.sport);
  LL_DBG_PRINTK("[ACL4] dport %d\n", key.dport);
  LL_DBG_PRINTK("[ACL4] l4proto %d\n", key.l4proto);

  xf->pm.table_id = LL_DP_ACLV4_MAP;

  act = bpf_map_lookup_elem(&acl_v4_map, &key);
  if (!act) {
    LL_DBG_PRINTK("[ACL4] miss");
    goto ct_trk;
  }

  xf->pm.phit |= LLB_DP_ACL_HIT;
  act->lts = bpf_ktime_get_ns();

#ifdef HAVE_DP_FC
  fa->ca.cidx = act->ca.cidx;
#endif

  if (act->ca.act_type == DP_SET_DO_CT) {
    goto ct_trk;
  } else if (act->ca.act_type == DP_SET_NOP) {
    struct dp_rdr_act *ar = &act->port_act;
    if (xf->pm.l4fin) {
      ar->fr = 1;
    }

    if (ar->fr == 1) {
      goto ct_trk;
    }

  } else if (act->ca.act_type == DP_SET_RDR_PORT) {
    struct dp_rdr_act *ar = &act->port_act;

    if (xf->pm.l4fin) {
      ar->fr = 1;
    }

    if (ar->fr == 1) {
      goto ct_trk;
    }

    LLBS_PPLN_RDR_PRIO(xf);
    xf->pm.oport = ar->oport;
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

    if (xf->pm.l4fin) {
      na->fr = 1;
    }

    dp_pipe_set_nat(ctx, xf, na, act->ca.act_type == DP_SET_SNAT ? 1: 0);
    dp_do_map_stats(ctx, xf, LL_DP_NAT4_STATS_MAP, na->rid);

    if (na->fr == 1 || na->doct) {
      goto ct_trk;
    }

  } else if (act->ca.act_type == DP_SET_TOCP) {
    /*LLBS_PPLN_TRAP(xf);*/
    LLBS_PPLN_TRAPC(xf, LLB_PIPE_RC_ACL_MISS);
  } else if (act->ca.act_type == DP_SET_SESS_FWD_ACT) {
    struct dp_sess_act *pa = &act->pdr_sess_act; 
    xf->pm.sess_id = pa->sess_id;
  } else {
    /* Same for DP_SET_DROP */
    LLBS_PPLN_DROP(xf);
  }

  dp_do_map_stats(ctx, xf, LL_DP_ACLV4_STATS_MAP, act->ca.cidx);
#if 0
  /* Note that this might result in consistency problems 
   * between packet and byte counts at times but this should be 
   * better than holding bpf-spinlock 
   */
  lock_xadd(&act->ctd.pb.bytes, xf->pm.l3_len);
  lock_xadd(&act->ctd.pb.packets, 1);
#endif

  return 0;

ct_trk:
  return dp_tail_call(ctx, xf, fa_, LLB_DP_CT_PGM_ID);
}

static void __always_inline
dp_do_ipv4_fwd(void *ctx,  struct xfi *xf, void *fa_)
{
  if (xf->tm.tunnel_id == 0 ||  xf->tm.tun_type != LLB_TUN_GTP) {
    dp_do_sess4_lkup(ctx, xf);
  }

  if (xf->pm.phit & LLB_DP_TMAC_HIT) {

    /* If some pipeline block already set a redirect before this,
     * we honor this and dont do further l3 processing 
     */
    if ((xf->pm.pipe_act & LLB_PIPE_RDR_MASK) == 0) {
      dp_do_rtv4_lkup(ctx, xf, fa_);
    }
  }
}

static int __always_inline
dp_ing_ipv4(void *ctx,  struct xfi *xf, void *fa_)
{
  if (xf->tm.tunnel_id && xf->tm.tun_type == LLB_TUN_GTP) {
    dp_do_sess4_lkup(ctx, xf);
  }
  dp_do_aclv4_lkup(ctx, xf, fa_);
  dp_do_ipv4_fwd(ctx, xf, fa_);

  return 0;
}
