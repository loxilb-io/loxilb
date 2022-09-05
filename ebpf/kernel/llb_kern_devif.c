/*
 *  llb_kernel_devif.c: LoxiLB kernel eBPF dev in/out pipeline
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int __always_inline
dp_do_if_lkup(void *ctx, struct xfi *xf)
{
  struct intf_key key;
  struct dp_intf_tact *l2a;

  key.ifindex = DP_IFI(ctx);
  key.ing_vid = xf->l2m.vlan[0];
  key.pad =  0;

  LL_DBG_PRINTK("[INTF] -- Lookup\n");
  LL_DBG_PRINTK("[INTF] ifidx %d vid %d\n",
                key.ifindex, bpf_ntohs(key.ing_vid));
  
  xf->pm.table_id = LL_DP_SMAC_MAP;

  l2a = bpf_map_lookup_elem(&intf_map, &key);
  if (!l2a) {
    //LLBS_PPLN_DROP(xf);
    LL_DBG_PRINTK("[INTF] not found");
    LLBS_PPLN_PASS(xf);
    return -1;
  }

  LL_DBG_PRINTK("[INTF] L2 action %d\n", l2a->ca.act_type);

  if (l2a->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(xf);
  } else if (l2a->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(xf);
  } else if (l2a->ca.act_type == DP_SET_IFI) {
    xf->pm.iport = l2a->set_ifi.xdp_ifidx;
    xf->pm.zone  = l2a->set_ifi.zone;
    xf->pm.bd    = l2a->set_ifi.bd;
    xf->pm.mirr  = l2a->set_ifi.mirr;
    xf->pm.pprop = l2a->set_ifi.pprop;
    xf->qm.polid = l2a->set_ifi.polid;
  } else {
    LLBS_PPLN_DROP(xf);
  }

  return 0;
}

#ifdef LL_TC_EBPF
static int __always_inline
dp_do_mark_mirr(void *ctx, struct xfi *xf)
{
  struct __sk_buff *skb = DP_TC_PTR(ctx);
  int *oif;
  int key;

  key = LLB_PORT_NO;
  oif = bpf_map_lookup_elem(&tx_intf_map, &key);
  if (!oif) {
    return -1;
  }

  skb->cb[0] = LLB_MIRR_MARK;
  skb->cb[1] = xf->pm.mirr; 

  LL_DBG_PRINTK("[REDR] Mirr port %d OIF %d\n", key, *oif);
  return bpf_clone_redirect(skb, *oif, BPF_F_INGRESS);
}

static int
dp_do_mirr_lkup(void *ctx, struct xfi *xf)
{
  struct dp_mirr_tact *ma;
  __u32 mkey = xf->pm.mirr;

  LL_DBG_PRINTK("[MIRR] -- Lookup\n");
  LL_DBG_PRINTK("[MIRR] -- Key %u\n", mkey);

  ma = bpf_map_lookup_elem(&mirr_map, &mkey);
  if (!ma) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  LL_DBG_PRINTK("[MIRR] Action %d\n", ma->ca.act_type);

  if (ma->ca.act_type == DP_SET_ADD_L2VLAN ||
      ma->ca.act_type == DP_SET_RM_L2VLAN) {
    struct dp_l2vlan_act *va = &ma->vlan_act;
    return dp_set_egr_vlan(ctx, xf,
                    ma->ca.act_type == DP_SET_RM_L2VLAN ?
                    0 : va->vlan, va->oport);
  }
  /* VXLAN to be done */

  LLBS_PPLN_DROP(xf);
  return -1;
}

#else

static int __always_inline
dp_do_mark_mirr(void *ctx, struct xfi *xf)
{
  return 0;
}

static int __always_inline
dp_do_mirr_lkup(void *ctx, struct xfi *xf)
{
  return 0;

}
#endif

#ifdef LLB_TRAP_PERF_RING
static int __always_inline
dp_trap_packet(void *ctx,  struct xfi *xf)
{
  struct ll_dp_pmdi *pmd;
  int z = 0;
  __u64 flags = BPF_F_CURRENT_CPU;

  /* Metadata will be in the perf event before the packet data. */
  pmd = bpf_map_lookup_elem(&pkts, &z);
  if (!pmd) return 0;

  LL_DBG_PRINTK("[TRAP] START--\n");

  pmd->ifindex = ctx->ingress_ifindex;
  pmd->xdp_inport = xf->pm.iport;
  pmd->xdp_oport = xf->pm.oport;
  pmd->pm.table_id = xf->table_id;
  pmd->rcode = xf->pm.rcode;
  pmd->pkt_len = xf->pm.py_bytes;

  flags |= (__u64)pmd->pkt_len << 32;
  
  if (bpf_perf_event_output(ctx, &pkt_ring, flags,
                            pmd, sizeof(*pmd))) {
    LL_DBG_PRINTK("[TRAP] FAIL--\n");
  }
  return DP_DROP;
}
#else
static int __always_inline
dp_trap_packet(void *ctx,  struct xfi *xf, void *fa_)
{
  struct ethhdr *neth;
  struct ethhdr *oeth;
  uint16_t ntype;
  struct llb_ethheader *llb;
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  LL_DBG_PRINTK("[TRAP] START--\n");

  /* FIXME - There is a problem right now if we send decapped
   * packet up the stack. So, this is a safety check for now
   */
  //if (xf->tm.tun_decap)
  //  return DP_DROP;

  oeth = DP_TC_PTR(DP_PDATA(ctx));
  if (oeth + 1 > dend) {
    return DP_DROP;
  }

  /* If tunnel was present, outer metadata is popped */
  memcpy(xf->l2m.dl_dst, oeth->h_dest, 6*2);
  ntype = oeth->h_proto;

  if (dp_add_l2(ctx, (int)sizeof(*llb))) {
    /* This can fail to push headroom for tunnelled packets.
     * It might be better to pass it rather than drop it in case
     * of failure
     */
    return DP_PASS;
  }

  neth = DP_TC_PTR(DP_PDATA(ctx));
  dend = DP_TC_PTR(DP_PDATA_END(ctx));
  if (neth + 1 > dend) {
    return DP_DROP;
  }

  memcpy(neth->h_dest, xf->l2m.dl_dst, 6*2);
  neth->h_proto = bpf_htons(ETH_TYPE_LLB); 
  
  /* Add LLB shim */
  llb = DP_ADD_PTR(neth, sizeof(*neth));
  if (llb + 1 > dend) {
    return DP_DROP;
  }

  llb->iport = bpf_htons(xf->pm.iport);
  llb->oport = bpf_htons(xf->pm.oport);
  llb->rcode = xf->pm.rcode;
  if (xf->tm.tun_decap) {
    llb->rcode |= LLB_PIPE_RC_TUN_DECAP;
  }
  llb->miss_table = xf->pm.table_id; /* FIXME */
  llb->next_eth_type = ntype;

  xf->pm.oport = LLB_PORT_NO;
  if (dp_redirect_port(&tx_intf_map, xf) != DP_REDIRECT) {
    LL_DBG_PRINTK("[TRAP] FAIL--\n");
    return DP_DROP;
  }

  /* TODO - Apply stats */
  return DP_REDIRECT;
}
#endif

static int __always_inline
dp_unparse_packet_always(void *ctx,  struct xfi *xf)
{

  if (xf->pm.nf & LLB_NAT_SRC) {
    LL_DBG_PRINTK("[DEPR] LL_SNAT 0x%lx:%x\n",
                 xf->l4m.nxip, xf->l4m.nxport);
    if (dp_do_snat(ctx, xf, xf->l4m.nxip, xf->l4m.nxport) != 0) {
      return DP_DROP;
    }
  } else if (xf->pm.nf & LLB_NAT_DST) {
    LL_DBG_PRINTK("[DEPR] LL_DNAT 0x%x\n",
                  xf->l4m.nxip, xf->l4m.nxport);
    if (dp_do_dnat(ctx, xf, xf->l4m.nxip, xf->l4m.nxport) != 0) {
      return DP_DROP;
    }
  }

  if (xf->tm.tun_decap) {
    if (xf->tm.tun_type == LLB_TUN_GTP) {
      LL_DBG_PRINTK("[DEPR] LL STRIP-GTP\n");
      if (dp_do_strip_gtp(ctx, xf, xf->pm.tun_off) != 0) {
        return DP_DROP;
      }
    }
  } else if (xf->tm.new_tunnel_id) {
    if (xf->tm.tun_type == LLB_TUN_GTP) {
      if (dp_do_ins_gtp(ctx, xf,
                        xf->tm.tun_rip,
                        xf->tm.tun_sip,
                        xf->tm.new_tunnel_id,
                        xf->qm.qfi,
                        1)) {
        return DP_DROP;
      }
    }
  }

  return 0;
}

static int __always_inline
dp_unparse_packet(void *ctx,  struct xfi *xf)
{
  if (xf->tm.tun_decap) {
    if (xf->tm.tun_type == LLB_TUN_VXLAN) {
      LL_DBG_PRINTK("[DEPR] LL STRIP-VXLAN\n");
      if (dp_do_strip_vxlan(ctx, xf, xf->pm.tun_off) != 0) {
        return DP_DROP;
      }
    }
  } else if (xf->tm.new_tunnel_id) {
    LL_DBG_PRINTK("[DEPR] LL_NEW-TUN 0x%x\n",
                  bpf_ntohl(xf->tm.new_tunnel_id));
    if (xf->tm.tun_type == LLB_TUN_VXLAN) {
      if (dp_do_ins_vxlan(ctx, xf,
                          xf->tm.tun_rip,
                          xf->tm.tun_sip, 
                          xf->tm.new_tunnel_id,
                          1)) {
        return DP_DROP;
      }
    }
  }

  return dp_do_out_vlan(ctx, xf);
}

static int __always_inline
dp_redir_packet(void *ctx,  struct xfi *xf)
{
  LL_DBG_PRINTK("[REDI] --\n");

  if (dp_redirect_port(&tx_intf_map, xf) != DP_REDIRECT) {
    LL_DBG_PRINTK("[REDI] FAIL--\n");
    return DP_DROP;
  }

#ifdef LLB_DP_IF_STATS
  dp_do_map_stats(ctx, xf, LL_DP_TX_INTF_STATS_MAP, xf->pm.oport);
#endif

  return DP_REDIRECT;
}

static int __always_inline
dp_rewire_packet(void *ctx,  struct xfi *xf)
{
  LL_DBG_PRINTK("[REWR] --\n");

  if (dp_rewire_port(&tx_intf_map, xf) != DP_REDIRECT) {
    LL_DBG_PRINTK("[REWR] FAIL--\n");
    return DP_DROP;
  }

  return DP_REDIRECT;
}

#ifdef HAVE_DP_FUNCS
static int
#else
static int __always_inline
#endif
dp_pipe_check_res(void *ctx, struct xfi *xf, void *fa)
{
  LL_DBG_PRINTK("[PIPE] act 0x%x\n", xf->pm.pipe_act);

  if (xf->pm.pipe_act) {

    if (xf->pm.pipe_act & LLB_PIPE_DROP) {
      return DP_DROP;
    } 

    if (dp_unparse_packet_always(ctx, xf) != 0) {
        return DP_DROP;
    }

#ifndef HAVE_LLB_DISAGGR
#ifdef HAVE_OOB_CH
    if (xf->pm.pipe_act & LLB_PIPE_TRAP) { 
      return dp_trap_packet(ctx, xf, fa);
    } 

    if (xf->pm.pipe_act & LLB_PIPE_PASS) {
#else
    if (xf->pm.pipe_act & (LLB_PIPE_TRAP | LLB_PIPE_PASS)) {
#endif
      return DP_PASS;
    }
#else
    if (xf->pm.pipe_act & (LLB_PIPE_TRAP | LLB_PIPE_PASS)) { 
      return dp_trap_packet(ctx, xf, fa);
    } 
#endif

    if (xf->pm.pipe_act & LLB_PIPE_RDR_MASK) {
      if (dp_unparse_packet(ctx, xf) != 0) {
        return DP_DROP;
      }
      return dp_redir_packet(ctx, xf);
    }

  } 
  return DP_PASS; /* FIXME */
}

static int __always_inline
dp_ing(void *ctx,  struct xfi *xf)
{
  dp_do_if_lkup(ctx, xf);
#ifdef LLB_DP_IF_STATS
  dp_do_map_stats(ctx, xf, LL_DP_INTF_STATS_MAP, xf->pm.iport);
#endif
  dp_do_map_stats(ctx, xf, LL_DP_BD_STATS_MAP, xf->pm.bd);

  if (xf->pm.mirr != 0) {
    dp_do_mark_mirr(ctx, xf);
  }

  if (xf->qm.polid != 0) {
    do_dp_policer(ctx, xf);
  }

  return 0;
}

static int __always_inline
dp_insert_fcv4(void *ctx, struct xfi *xf, struct dp_fc_tacts *acts)
{
  struct dp_fcv4_key *key;
  int z = 0;
  int *oif;
  int pkey = xf->pm.oport;
  
  oif = bpf_map_lookup_elem(&tx_intf_map, &pkey);
  if (oif) {
    acts->ca.oif = *oif;
  } 

  LL_DBG_PRINTK("[FCH4] INS--\n");

  key = bpf_map_lookup_elem(&xfck, &z);
  if (key == NULL) {
    return -1;
  }

  if (bpf_map_lookup_elem(&fc_v4_map, key) != NULL) {
    return 1;
  }
  
  bpf_map_update_elem(&fc_v4_map, key, acts, BPF_ANY);
  return 0;
}

static int __always_inline
dp_ing_slow_main(void *ctx,  struct xfi *xf)
{
  struct dp_fc_tacts *fa = NULL;
#ifdef HAVE_DP_FC
  int z = 0;

  fa = bpf_map_lookup_elem(&fcas, &z);
  if (!fa) return 0;

  /* No nonsense no loop */
  fa->ca.ftrap = 0;
  fa->ca.cidx = 0;
  fa->its = bpf_ktime_get_ns();
  fa->fcta[0].ca.act_type = 0;
  fa->fcta[1].ca.act_type = 0;
  fa->fcta[2].ca.act_type = 0;
  fa->fcta[3].ca.act_type = 0;
  fa->fcta[4].ca.act_type = 0;
  fa->fcta[5].ca.act_type = 0;
  fa->fcta[6].ca.act_type = 0;
  fa->fcta[7].ca.act_type = 0;
  fa->fcta[8].ca.act_type = 0;
  fa->fcta[9].ca.act_type = 0; // LLB_FCV4_MAP_ACTS -1 

  /* memset is too costly */
  /*memset(fa->fcta, 0, sizeof(fa->fcta));*/
#endif

  LL_DBG_PRINTK("[INGR] START--\n");

  /* If there are any packets marked for mirroring, we do
   * it here and immediately get it out of way without
   * doing any further processing
   */
  if (xf->pm.mirr != 0) {
    dp_do_mirr_lkup(ctx, xf);
    goto out;
  }

  dp_ing(ctx, xf);

  /* If there are pipeline errors at this stage,
   * we again skip any further processing
   */
  if (xf->pm.pipe_act || xf->pm.tc == 0) {
    goto out;
  }

  dp_ing_l2(ctx, xf, fa);

#ifdef HAVE_DP_FC
  /* fast-cache is used only when certain conditions are met */
  if (xf->pm.pipe_act == LLB_PIPE_RDR && 
      xf->pm.phit & LLB_DP_ACL_HIT &&
      !(xf->pm.phit & LLB_DP_SESS_HIT) &&
      xf->qm.polid == 0 &&
      xf->pm.mirr == 0) {
    dp_insert_fcv4(ctx, xf, fa);
  }
#endif
out:
  return dp_pipe_check_res(ctx, xf, fa);
}

static int __always_inline
dp_ing_ct_main(void *ctx,  struct xfi *xf)
{
  int val = 0;
  struct dp_fc_tacts *fa = NULL;

#ifdef HAVE_DP_FC
  fa = bpf_map_lookup_elem(&fcas, &val);
  if (!fa) return DP_DROP;
#endif

  /* If ACL is hit, and packet arrives here 
   * it only means that we need CT processing.
   * In such a case, we skip nat lookup
   */
  if ((xf->pm.phit & LLB_DP_ACL_HIT) == 0)
    dp_do_nat4_rule_lkup(ctx, xf);

  LL_DBG_PRINTK("[CTRK] start\n");

  val = dp_ctv4_in(ctx, xf);
  if (val < 0) {
    return DP_PASS;
  }

  xf->l4m.ct_sts = LLB_PIPE_CT_INP;

  /* CT pipeline is hit after acl lookup fails 
   * So, after CT processing we continue the rest
   * of the stack. We could potentially make 
   * another tail-call to where ACL lookup failed
   * and start over. But simplicity wins against
   * complexity for now 
   */
  if (xf->l2m.dl_type == bpf_htons(ETH_P_IP)) {
    dp_do_ipv4_fwd(ctx, xf, fa);
  }
  dp_eg_l2(ctx, xf, fa);
  return dp_pipe_check_res(ctx, xf, fa);
}
 
static int __always_inline
dp_ing_pass_main(void *ctx)
{
  LL_DBG_PRINTK("[INGR] PASS--\n");

  return DP_PASS;
}
