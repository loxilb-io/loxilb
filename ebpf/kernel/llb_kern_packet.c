/*
 *  llb_kernel_packet.c: LoxiLB kernel eBPF packet pipeline 
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int __always_inline
dp_do_if_lkup(void *ctx, struct xfi *F)
{
  struct intf_key key;
  struct dp_intf_tact *l2a;

  key.ifindex = DP_IFI(ctx);
  key.ing_vid = F->l2m.vlan[0];
  key.pad =  0;

  LL_DBG_PRINTK("[INTF] -- Lookup\n");
  LL_DBG_PRINTK("[INTF] ifidx %d vid %d\n",
                key.ifindex, bpf_ntohs(key.ing_vid));
  
  F->pm.table_id = LL_DP_SMAC_MAP;

  l2a = bpf_map_lookup_elem(&intf_map, &key);
  if (!l2a) {
    //LLBS_PPLN_DROP(F);
    LL_DBG_PRINTK("[INTF] not found");
    LLBS_PPLN_PASS(F);
    return -1;
  }

  LL_DBG_PRINTK("[INTF] L2 action %d\n", l2a->ca.act_type);

  if (l2a->ca.act_type == DP_SET_DROP) {
    LLBS_PPLN_DROP(F);
  } else if (l2a->ca.act_type == DP_SET_TOCP) {
    LLBS_PPLN_TRAP(F);
  } else if (l2a->ca.act_type == DP_SET_IFI) {
    F->pm.iport = l2a->set_ifi.xdp_ifidx;
    F->pm.zone  = l2a->set_ifi.zone;
    F->pm.bd    = l2a->set_ifi.bd;
    F->pm.mirr  = l2a->set_ifi.mirr;
    F->pm.pprop = l2a->set_ifi.pprop;
    F->qm.polid = l2a->set_ifi.polid;
  } else {
    LLBS_PPLN_DROP(F);
  }

  return 0;
}

#ifdef LL_TC_EBPF
static int __always_inline
dp_do_mark_mirr(void *ctx, struct xfi *F)
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
  skb->cb[1] = F->pm.mirr; 
  F->pm.mirr = 0;

  LL_DBG_PRINTK("[REDR] Mirr port %d OIF %d\n", key, *oif);
  return bpf_clone_redirect(skb, *oif, BPF_F_INGRESS);
}

static int
dp_do_mirr_lkup(void *ctx, struct xfi *F)
{
  struct dp_mirr_tact *ma;
  __u32 mkey = F->pm.mirr;

  LL_DBG_PRINTK("[MIRR] -- Lookup\n");
  LL_DBG_PRINTK("[MIRR] -- Key %u\n", mkey);

  ma = bpf_map_lookup_elem(&mirr_map, &mkey);
  if (!ma) {
    LLBS_PPLN_DROP(F);
    return -1;
  }

  LL_DBG_PRINTK("[MIRR] Action %d\n", ma->ca.act_type);

  if (ma->ca.act_type == DP_SET_ADD_L2VLAN ||
      ma->ca.act_type == DP_SET_RM_L2VLAN) {
    struct dp_l2vlan_act *va = &ma->vlan_act;
    return dp_set_egr_vlan(ctx, F,
                    ma->ca.act_type == DP_SET_RM_L2VLAN ?
                    0 : va->vlan, va->oport);
  }
  /* VXLAN to be done */

  LLBS_PPLN_DROP(F);
  return -1;
}

#else

static int __always_inline
dp_do_mark_mirr(void *ctx, struct xfi *F)
{
  
  return 0;
}

static int __always_inline
dp_do_mirr_lkup(void *ctx, struct xfi *F)
{
  return 0;

}
#endif

#ifdef LLB_TRAP_PERF_RING
static int __always_inline
dp_trap_packet(void *ctx,  struct xfi *F)
{
  struct ll_dp_pmdi *pmd;
  int z = 0;
  __u64 flags = BPF_F_CURRENT_CPU;

  /* Metadata will be in the perf event before the packet data. */
  pmd = bpf_map_lookup_elem(&pkts, &z);
  if (!pmd) return 0;

  LL_DBG_PRINTK("[TRAP] START--\n");

  pmd->ifindex = ctx->ingress_ifindex;
  pmd->xdp_inport = F->pm.iport;
  pmd->xdp_oport = F->pm.oport;
  pmd->pm.table_id = F->table_id;
  pmd->rcode = F->pm.rcode;
  pmd->pkt_len = F->pm.py_bytes;

  flags |= (__u64)pmd->pkt_len << 32;
  
  if (bpf_perf_event_output(ctx, &pkt_ring, flags,
                            pmd, sizeof(*pmd))) {
    LL_DBG_PRINTK("[TRAP] FAIL--\n");
  }
  return DP_DROP;
}
#else
static int __always_inline
dp_trap_packet(void *ctx,  struct xfi *F, void *fa_)
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
  //if (F->tm.tun_decap)
  //  return DP_DROP;

  oeth = DP_TC_PTR(DP_PDATA(ctx));
  if (oeth + 1 > dend) {
    return DP_DROP;
  }

  /* If tunnel was present, outer metadata is popped */
  memcpy(F->l2m.dl_dst, oeth->h_dest, 6*2);
  ntype = oeth->h_proto;

  if (dp_add_l2(ctx, (int)sizeof(*llb))) {
    /* Note : This func usually fails to push headroom for VxLAN packets but we
     * can't drop those packets here. We will pass them on to Linux(SLow path)*/
    return DP_PASS;
  }

  neth = DP_TC_PTR(DP_PDATA(ctx));
  dend = DP_TC_PTR(DP_PDATA_END(ctx));
  if (neth + 1 > dend) {
    return DP_DROP;
  }

  memcpy(neth->h_dest, F->l2m.dl_dst, 6*2);
  neth->h_proto = bpf_htons(ETH_TYPE_LLB); 
  
  /* Add LLB shim */
  llb = DP_ADD_PTR(neth, sizeof(*neth));
  if (llb + 1 > dend) {
    return DP_DROP;
  }

  llb->iport = bpf_htons(F->pm.iport);
  llb->oport = bpf_htons(F->pm.oport);
  llb->rcode = F->pm.rcode;
  if (F->tm.tun_decap) {
    llb->rcode |= LLB_PIPE_RC_TUN_DECAP;
  }
  llb->miss_table = F->pm.table_id; /* FIXME */
  llb->next_eth_type = ntype;

  F->pm.oport = LLB_PORT_NO;
  if (dp_redirect_port(&tx_intf_map, F) != DP_REDIRECT) {
    LL_DBG_PRINTK("[TRAP] FAIL--\n");
    return DP_DROP;
  }

  /* TODO - Apply stats */
  return DP_REDIRECT;
}
#endif

static int __always_inline
dp_unparse_packet(void *ctx,  struct xfi *F)
{
  if (F->tm.tun_decap) {
    if (F->tm.tun_type == LLB_TUN_VXLAN) {
      LL_DBG_PRINTK("[DEPR] LL STRIP-VXLAN\n");
      if (dp_do_strip_vxlan(ctx, F, F->pm.tun_off) != 0) {
        return DP_DROP;
      }
    } else if (F->tm.tun_type == LLB_TUN_GTP) {
      LL_DBG_PRINTK("[DEPR] LL STRIP-GTP\n");
      if (dp_do_strip_gtp(ctx, F, F->pm.tun_off) != 0) {
        return DP_DROP;
      }
    }
  }

  if (F->pm.nf & LLB_NAT_SRC) {
    LL_DBG_PRINTK("[DEPR] LL_SNAT 0x%lx:%x\n",
                 F->l4m.nxip, F->l4m.nxport);
    if (dp_do_snat(ctx, F, F->l4m.nxip, F->l4m.nxport) != 0) {
      return DP_DROP;
    }
  } else if (F->pm.nf & LLB_NAT_DST) {
    LL_DBG_PRINTK("[DEPR] LL_DNAT 0x%x\n",
                  F->l4m.nxip, F->l4m.nxport);
    if (dp_do_dnat(ctx, F, F->l4m.nxip, F->l4m.nxport) != 0) {
      return DP_DROP;
    }
  }

  if (F->tm.new_tunnel_id) {
    LL_DBG_PRINTK("[DEPR] LL_NEW-TUN 0x%x\n",
                  bpf_ntohl(F->tm.new_tunnel_id));
    if (dp_do_ins_vxlan(ctx, F,
                          F->tm.tun_rip,
                          F->tm.tun_sip, 
                          F->tm.new_tunnel_id,
                          1)) {
      return DP_DROP;
    }
  }

  return dp_do_out_vlan(ctx, F);
}

static int __always_inline
dp_redir_packet(void *ctx,  struct xfi *F)
{
  LL_DBG_PRINTK("[REDI] --\n");

  if (dp_redirect_port(&tx_intf_map, F) != DP_REDIRECT) {
    LL_DBG_PRINTK("[REDI] FAIL--\n");
    return DP_DROP;
  }

#ifdef LLB_XDP_IF_STATS
  dp_do_map_stats(ctx, F, LL_DP_TX_INTF_STATS_MAP, F->pm.oport);
#endif

  return DP_REDIRECT;
}

static int __always_inline
dp_rewire_packet(void *ctx,  struct xfi *F)
{
  LL_DBG_PRINTK("[REWR] --\n");

  if (dp_rewire_port(&tx_intf_map, F) != DP_REDIRECT) {
    LL_DBG_PRINTK("[REWR] FAIL--\n");
    return DP_DROP;
  }

  return DP_REDIRECT;
}

static int
dp_pipe_check_res(void *ctx, struct xfi *F, void *fa)
{
  LL_DBG_PRINTK("[PIPE] act 0x%x\n", F->pm.pipe_act);
  
  if (F->pm.pipe_act) {

    if (F->pm.pipe_act & LLB_PIPE_DROP) {
      return DP_DROP;
    } 

    //if (F->pm.pipe_act & LLB_PIPE_SET_CT) {
    //  return dp_tail_call(ctx, F, LLB_DP_CT_PGM_ID);
    //}

#ifndef HAVE_LLB_DISAGGR
#ifdef HAVE_OOB_CH
    if (F->pm.pipe_act & LLB_PIPE_TRAP) { 
      return dp_trap_packet(ctx, F, fa);
    } 

    if (F->pm.pipe_act & LLB_PIPE_PASS) {
#else
    if (F->pm.pipe_act & (LLB_PIPE_TRAP | LLB_PIPE_PASS)) {
#endif
      return DP_PASS;
    }
#else
    if (F->pm.pipe_act & (LLB_PIPE_TRAP | LLB_PIPE_PASS)) { 
      return dp_trap_packet(ctx, F, fa);
    } 
#endif

    if (F->pm.pipe_act & LLB_PIPE_RDR_MASK) {
      if (dp_unparse_packet(ctx, F) != 0) {
        return DP_DROP;
      }
      return dp_redir_packet(ctx, F);
    }

  } 
  return DP_PASS; /* FIXME */
}

static int __always_inline
dp_ing(void *ctx,  struct xfi *F)
{
  if (F->pm.mirr != 0) {
    dp_do_mirr_lkup(ctx, F); 
    return 0;
  }

  dp_do_if_lkup(ctx, F);
#ifdef LLB_XDP_IF_STATS
  dp_do_map_stats(ctx, F, LL_DP_INTF_STATS_MAP, F->pm.iport);
#endif
  dp_do_map_stats(ctx, F, LL_DP_BD_STATS_MAP, F->pm.bd);

  if (F->pm.mirr != 0) {
    dp_do_mark_mirr(ctx, F);
  }

  if (F->qm.polid != 0) {
    do_dp_policer(ctx, F);
  }

  return 0;
}

#define dp_ing_mirror(ctx, F)    \
do {                              \
  if (F->pm.mirr != 0)            \
    dp_do_mirr_lkup(ctx, F);  \
}while(0);

static int __always_inline
dp_insert_fcv4(void *ctx, struct xfi *F, struct dp_fc_tacts *acts)
{
  struct dp_fcv4_key *key;
  int z = 0;
  int *oif;
  int pkey = F->pm.oport;
  
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
dp_ing_slow_main(void *ctx,  struct xfi *F)
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
#endif

  LL_DBG_PRINTK("[INGR] START--\n");
  dp_ing(ctx, F);

  if (F->pm.pipe_act || F->pm.tc == 0) {
    LL_DBG_PRINTK("[INGR] Go out--\n");
    goto out;
  }
  dp_ing_l2(ctx, F, fa);
out:
#ifdef HAVE_DP_FC
  if (F->pm.pipe_act == LLB_PIPE_RDR && 
      F->pm.phit & LLB_DP_ACL_HIT &&
      F->qm.polid == 0 &&
      !DP_NEED_MIRR(ctx)) {
    dp_insert_fcv4(ctx, F, fa);
  }
#endif
  return dp_pipe_check_res(ctx, F, fa);
}

static int __always_inline
dp_ing_ct_main(void *ctx,  struct xfi *F)
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
  if ((F->pm.phit & LLB_DP_ACL_HIT) == 0)
    dp_do_nat4_rule_lkup(ctx, F);

  LL_DBG_PRINTK("[CTRK] start\n");
  bpf_printk("[CTRK] start\n");

  val = dp_ctv4_in(ctx, F);
  if (val < 0) {
    return DP_DROP;
  }

  F->l4m.ct_sts = LLB_PIPE_CT_INP;

  /* CT pipeline is hit after acl lookup fails 
   * So, after CT processing we continue the rest
   * of the stack. We could potentially make 
   * another tail-call to where ACL lookup failed
   * and start over. But simplicity wins against
   * complexity for now 
   */
  if (F->l2m.dl_type == bpf_htons(ETH_P_IP)) {
    dp_do_ipv4_fwd(ctx, F, fa);
  }
  dp_eg_l2(ctx, F, fa);
  return dp_pipe_check_res(ctx, F, fa);
}
 
static int __always_inline
dp_ing_pass_main(void *ctx)
{
  LL_DBG_PRINTK("[INGR] PASS--\n");

  return DP_PASS;
}
