/*
 *  llb_kern_l2fwd.c: LoxiLB kernel eBPF L2 forwarder Implementation
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int __always_inline
dp_do_smac_lkup(void *ctx, struct xfi *xf, void *fc)
{
  struct dp_smac_key key;
  struct dp_smac_tact *sma;

  if (xf->l2m.vlan[0] == 0) {
    return 0;
  }

  memcpy(key.smac, xf->l2m.dl_src, 6);
  key.bd = xf->pm.bd;

  LL_DBG_PRINTK("[SMAC] -- Lookup\n");
  LL_DBG_PRINTK("[SMAC] %x:%x:%x\n",
                 key.smac[0], key.smac[1], key.smac[2]);
  LL_DBG_PRINTK("[SMAC] %x:%x:%x\n",
                 key.smac[3], key.smac[4], key.smac[5]);
  LL_DBG_PRINTK("[SMAC] BD%d\n", key.bd);

  xf->pm.table_id = LL_DP_SMAC_MAP;

  sma = bpf_map_lookup_elem(&smac_map, &key);
  if (!sma) {
    /* Default action */
    LLBS_PPLN_PASS(xf);
    return 0;
  }

  LL_DBG_PRINTK("[SMAC] action %d\n", sma->ca.act_type);

  if (sma->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(xf);
  } else if (sma->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(xf);
  } else if (sma->ca.act_type == DP_SET_NOP) {
    /* Nothing to do */
    return 0;
  } else {
    LLBS_PPLN_DROP(xf);
  }

  return 0;
}

static int __always_inline
dp_pipe_set_l22_tun_nh(void *ctx, struct xfi *xf, struct dp_rt_nh_act *rnh)
{
  xf->pm.nh_num = rnh->nh_num;

  /*
   * We do not set out_bd here. After NH lookup match is
   * found and packet tunnel insertion is done, BD is set accordingly
   */
  /*xf->pm.bd = rnh->bd;*/
  xf->tm.new_tunnel_id = rnh->tid;
  LL_DBG_PRINTK("[TMAC] new-vx nh %u\n", xf->pm.nh_num);
  return 0;
}

static int __always_inline
dp_pipe_set_rm_vx_tun(void *ctx, struct xfi *xf, struct dp_rt_nh_act *rnh)
{
  xf->pm.phit &= ~LLB_DP_TMAC_HIT;
  xf->pm.bd = rnh->bd;

  LL_DBG_PRINTK("[TMAC] rm-vx newbd %d \n", xf->pm.bd);
  return dp_pop_outer_metadata(ctx, xf, 1);
}

static int __always_inline
__dp_do_tmac_lkup(void *ctx, struct xfi *xf,
                  int tun_lkup, void *fa_)
{
  struct dp_tmac_key key;
  struct dp_tmac_tact *tma;
#ifdef HAVE_DP_FC
  struct dp_fc_tacts *fa = fa_;
#endif

  memcpy(key.mac, xf->l2m.dl_dst, 6);
  key.pad  = 0;
  if (tun_lkup) {
    key.tunnel_id = xf->tm.tunnel_id;
    key.tun_type = xf->tm.tun_type;
  } else {
    key.tunnel_id = 0;
    key.tun_type  = 0;
  }

  LL_DBG_PRINTK("[TMAC] -- Lookup\n");
  LL_DBG_PRINTK("[TMAC] %x:%x:%x\n",
                 key.mac[0], key.mac[1], key.mac[2]);
  LL_DBG_PRINTK("[TMAC] %x:%x:%x\n",
                 key.mac[3], key.mac[4], key.mac[5]);
  LL_DBG_PRINTK("[TMAC] %x:%x\n", key.tunnel_id, key.tun_type);

  xf->pm.table_id = LL_DP_TMAC_MAP;

  tma = bpf_map_lookup_elem(&tmac_map, &key);
  if (!tma) {
    /* No L3 lookup */
    return 0;
  }

  LL_DBG_PRINTK("[TMAC] action %d %d\n", tma->ca.act_type, tma->ca.cidx);
  if (tma->ca.cidx != 0) {
    dp_do_map_stats(ctx, xf, LL_DP_TMAC_STATS_MAP, tma->ca.cidx);
  }

  if (tma->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(xf);
  } else if (tma->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(xf);
  } else if (tma->ca.act_type == DP_SET_RT_TUN_NH) {
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[DP_SET_RT_TUN_NH];
    ta->ca.act_type = DP_SET_RT_TUN_NH;
    memcpy(&ta->nh_act,  &tma->rt_nh, sizeof(tma->rt_nh));
#endif
    return dp_pipe_set_l22_tun_nh(ctx, xf, &tma->rt_nh);
  } else if (tma->ca.act_type == DP_SET_L3_EN) {
    xf->pm.phit |= LLB_DP_TMAC_HIT;
  } else if (tma->ca.act_type == DP_SET_RM_VXLAN) {
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[DP_SET_RM_VXLAN];
    ta->ca.act_type = DP_SET_RM_VXLAN;
    memcpy(&ta->nh_act,  &tma->rt_nh, sizeof(tma->rt_nh));
#endif
    return dp_pipe_set_rm_vx_tun(ctx, xf, &tma->rt_nh);
  }

  return 0;
}

static int __always_inline
dp_do_tmac_lkup(void *ctx, struct xfi *xf, void *fa)
{
  return __dp_do_tmac_lkup(ctx, xf, 0, fa);
}

static int __always_inline
dp_do_tun_lkup(void *ctx, struct xfi *xf, void *fa)
{
  if (xf->tm.tunnel_id != 0) {
    return __dp_do_tmac_lkup(ctx, xf, 1, fa);
  }
  return 0;
}

static int __always_inline
dp_set_egr_vlan(void *ctx, struct xfi *xf,
                __u16 vlan, __u16 oport)
{
  LLBS_PPLN_RDR(xf);
  xf->pm.oport = oport;
  xf->pm.bd = vlan;
  LL_DBG_PRINTK("[SETVLAN] OP %u V %u\n", oport, vlan);
  return 0;
}

static int __always_inline
dp_do_dmac_lkup(void *ctx, struct xfi *xf, void *fa_)
{
  struct dp_dmac_key key;
  struct dp_dmac_tact *dma;
#ifdef HAVE_DP_FC
  struct dp_fc_tacts *fa = fa_;
#endif

  memcpy(key.dmac, xf->pm.lkup_dmac, 6);
  key.bd = xf->pm.bd;
  xf->pm.table_id = LL_DP_DMAC_MAP;

  LL_DBG_PRINTK("[DMAC] -- Lookup \n");
  LL_DBG_PRINTK("[DMAC] %x:%x:%x\n",
                 key.dmac[0], key.dmac[1], key.dmac[2]);
  LL_DBG_PRINTK("[DMAC] %x:%x:%x\n", 
                 key.dmac[3], key.dmac[4], key.dmac[5]);
  LL_DBG_PRINTK("[DMAC] BD %d\n", key.bd);

  dma = bpf_map_lookup_elem(&dmac_map, &key);
  if (!dma) {
    /* No DMAC lookup */
    LL_DBG_PRINTK("[DMAC] not found\n");
    LLBS_PPLN_PASS(xf);
    return 0;
  }

  LL_DBG_PRINTK("[DMAC] action %d pipe %d\n",
                 dma->ca.act_type, xf->pm.pipe_act);

  if (dma->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(xf);
  } else if (dma->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(xf);
  } else if (dma->ca.act_type == DP_SET_RDR_PORT) {
    struct dp_rdr_act *ra = &dma->port_act;

    LLBS_PPLN_RDR(xf);
    xf->pm.oport = ra->oport;
    LL_DBG_PRINTK("[DMAC] oport %lu\n", xf->pm.oport);
    return 0;
  } else if (dma->ca.act_type == DP_SET_ADD_L2VLAN || 
             dma->ca.act_type == DP_SET_RM_L2VLAN) {
    struct dp_l2vlan_act *va = &dma->vlan_act;
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[
                          dma->ca.act_type == DP_SET_ADD_L2VLAN ?
                          DP_SET_ADD_L2VLAN : DP_SET_RM_L2VLAN];
    ta->ca.act_type = dma->ca.act_type;
    memcpy(&ta->l2ov,  va, sizeof(*va));
#endif
    return dp_set_egr_vlan(ctx, xf, 
                    dma->ca.act_type == DP_SET_RM_L2VLAN ?
                    0 : va->vlan, va->oport);
  }

  return 0;
}

#ifdef HAVE_DP_FUNCS
static int
#else
static int __always_inline
#endif
dp_do_rt_l2_nh(void *ctx, struct xfi *xf,
               struct dp_rt_l2nh_act *nl2)
{
  memcpy(xf->l2m.dl_dst, nl2->dmac, 6);
  memcpy(xf->l2m.dl_src, nl2->smac, 6);
  memcpy(xf->pm.lkup_dmac, nl2->dmac, 6);
  xf->pm.bd = nl2->bd;
 
  return nl2->rnh_num;
}

#ifdef HAVE_DP_FUNCS
static int
#else
static int __always_inline
#endif
dp_do_rt_l2_vxlan_nh(void *ctx, struct xfi *xf,
                     struct dp_rt_l2vxnh_act *nl2vx)
{
  struct dp_rt_l2nh_act *nl2;

  xf->tm.tun_rip = nl2vx->l3t.rip;
  xf->tm.tun_sip = nl2vx->l3t.sip;
  xf->tm.new_tunnel_id = nl2vx->l3t.tid;
  xf->tm.tun_type = LLB_TUN_VXLAN;

  memcpy(&xf->il2m, &xf->l2m, sizeof(xf->l2m));
  xf->il2m.vlan[0] = 0;

  nl2 = &nl2vx->l2nh;
  memcpy(xf->l2m.dl_dst, nl2->dmac, 6);
  memcpy(xf->l2m.dl_src, nl2->smac, 6);
  memcpy(xf->pm.lkup_dmac, nl2->dmac, 6);
  xf->pm.bd = nl2->bd;
 
  return 0;
}

static int __always_inline
dp_do_nh_lkup(void *ctx, struct xfi *xf, void *fa_)
{
  struct dp_nh_key key;
  struct dp_nh_tact *nha;
  int rnh = 0;
#ifdef HAVE_DP_FC
  struct dp_fc_tacts *fa = fa_;
#endif

  key.nh_num = (__u32)xf->pm.nh_num;

  LL_DBG_PRINTK("[NHFW] -- Lookup ID %d\n", key.nh_num);
  xf->pm.table_id = LL_DP_NH_MAP;

  nha = bpf_map_lookup_elem(&nh_map, &key);
  if (!nha) {
    /* No NH - Drop */
    LLBS_PPLN_TRAP(xf)
    return 0;
  }

  LL_DBG_PRINTK("[NHFW] action %d pipe %x\n",
                nha->ca.act_type, xf->pm.pipe_act);

  if (nha->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(xf);
  } else if (nha->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(xf);
  } else if (nha->ca.act_type == DP_SET_NEIGH_L2) {
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[DP_SET_NEIGH_L2];
    ta->ca.act_type = nha->ca.act_type;
    memcpy(&ta->nl2,  &nha->rt_l2nh, sizeof(nha->rt_l2nh));
#endif
    rnh = dp_do_rt_l2_nh(ctx, xf, &nha->rt_l2nh);
    /* Check if need to do recursive next-hop lookup */
    if (rnh != 0) {
      key.nh_num = (__u32)rnh;
      nha = bpf_map_lookup_elem(&nh_map, &key);
      if (!nha) {
        /* No NH - Trap */
        // LLBS_PPLN_DROP(xf); //
        LLBS_PPLN_TRAP(xf)
        return 0;
      }
    }
  } 

  if (nha->ca.act_type == DP_SET_NEIGH_VXLAN) {
#ifdef HAVE_DP_FC
    struct dp_fc_tact *ta = &fa->fcta[DP_SET_NEIGH_VXLAN];
    ta->ca.act_type = nha->ca.act_type;
    memcpy(&ta->nl2vx,  &nha->rt_l2vxnh, sizeof(nha->rt_l2vxnh));
#endif
    return dp_do_rt_l2_vxlan_nh(ctx, xf, &nha->rt_l2vxnh);
  }

  return 0;
}

static int __always_inline
dp_eg_l2(void *ctx,  struct xfi *xf, void *fa)
{
  /* Any processing based on results from L3 */
  if (xf->pm.pipe_act & LLB_PIPE_RDR_MASK) {
    return 0;
  }   
      
  if (xf->pm.nh_num != 0) {
    dp_do_nh_lkup(ctx, xf, fa);
  }

  dp_do_map_stats(ctx, xf, LL_DP_TX_BD_STATS_MAP, xf->pm.bd);

  dp_do_dmac_lkup(ctx, xf, fa);
  return 0;
}

static int __always_inline
dp_ing_fwd(void *ctx,  struct xfi *xf, void *fa)
{
  if (xf->l2m.dl_type == bpf_htons(ETH_P_IP)) {
    dp_ing_ipv4(ctx, xf, fa);
  }
  return dp_eg_l2(ctx, xf, fa);
}

static int __always_inline
dp_ing_l2_top(void *ctx,  struct xfi *xf, void *fa)
{
  dp_do_smac_lkup(ctx, xf, fa);
  dp_do_tmac_lkup(ctx, xf, fa);
  dp_do_tun_lkup(ctx, xf, fa);

  if (xf->tm.tun_decap) {
    /* FIXME Also need to check if L2 tunnel */
    dp_do_smac_lkup(ctx, xf, fa);
    dp_do_tmac_lkup(ctx, xf, fa);
  }

  return 0;
}

static int __always_inline
dp_ing_l2(void *ctx,  struct xfi *xf, void *fa)
{
  LL_DBG_PRINTK("[ING L2]");
  dp_ing_l2_top(ctx, xf, fa);
  return dp_ing_fwd(ctx, xf, fa);
}
