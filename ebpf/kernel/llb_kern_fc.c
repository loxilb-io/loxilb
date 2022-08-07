/*
 *  llb_kern_fc.c: LoxiLB kernel cache based forwarding
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int  __always_inline
dp_mk_fcv4_key(struct xfi *F, struct dp_fcv4_key *key)
{
  memcpy(key->smac, F->l2m.dl_src, 6);
  memcpy(key->dmac, F->l2m.dl_dst, 6);
  memcpy(key->in_smac, F->il2m.dl_src, 6);
  memcpy(key->in_dmac, F->il2m.dl_dst, 6);

  //key->bd         = F->pm.bd;
  key->bd         = 0; 
  key->daddr      = F->l3m.ip.daddr;
  key->saddr      = F->l3m.ip.saddr;
  key->sport      = F->l3m.source;
  key->dport      = F->l3m.dest;
  key->l4proto    = F->l3m.nw_proto;

  //key->in_port    = F->pm.iport;
  key->in_port    = 0;
  key->in_daddr   = F->il3m.ip.daddr;
  key->in_saddr   = F->il3m.ip.saddr;
  key->in_sport   = F->il3m.source;
  key->in_dport   = F->il3m.dest;
  key->in_l4proto = F->il3m.nw_proto;

  return 0;
}

static int __always_inline
dp_do_fcv4_lkup(void *ctx, struct xfi *F)
{
  struct dp_fcv4_key key;
  struct dp_fc_tacts *acts;
  struct dp_fc_tact *ta;
  int ret = 1;

  dp_mk_fcv4_key(F, &key);

  LL_FC_PRINTK("[FCH4] -- Lookup\n");
  LL_FC_PRINTK("[FCH4] key-sz %d\n", sizeof(key));
  LL_FC_PRINTK("[FCH4] daddr %x\n", key.daddr);
  LL_FC_PRINTK("[FCH4] saddr %x\n", key.saddr);
  LL_FC_PRINTK("[FCH4] sport %x\n", key.sport);
  LL_FC_PRINTK("[FCH4] dport %x\n", key.dport);
  LL_FC_PRINTK("[FCH4] l4proto %x\n", key.l4proto);
  LL_FC_PRINTK("[FCH4] idaddr %x\n", key.in_daddr);
  LL_FC_PRINTK("[FCH4] isaddr %x\n", key.in_saddr);
  LL_FC_PRINTK("[FCH4] isport %x\n", key.in_sport);
  LL_FC_PRINTK("[FCH4] idport %x\n", key.in_dport);
  LL_FC_PRINTK("[FCH4] il4proto %x\n", key.in_l4proto);

  F->pm.table_id = LL_DP_FCV4_MAP;
  acts = bpf_map_lookup_elem(&fc_v4_map, &key);
  if (!acts) {
    int z = 0;
    /* xfck - fcache key table is maintained so that 
     * there is no need to make fcv4 key again in
     * tail-call sections
     */
    bpf_map_update_elem(&xfck, &z, &key, BPF_ANY);
    return 0; 
  }

#ifdef HAVE_LL_FC_DPTO
  /* Check hard-timeout */ 
  if (bpf_ktime_get_ns() - acts->its > FC_V4_HTO) {
    bpf_map_delete_elem(&fc_v4_map, &key);
    return 0; 
  }
#endif

  LL_FC_PRINTK("[FCH4] key found act-sz %d\n", sizeof(struct dp_fc_tacts));

  if (acts->ca.ftrap)
    return 0; 

  if (acts->fcta[DP_SET_RM_VXLAN].ca.act_type == DP_SET_RM_VXLAN) {
    LL_FC_PRINTK("[FCH4] strip-vxlan-act\n");
    ta = &acts->fcta[DP_SET_RM_VXLAN];
    dp_pipe_set_rm_vx_tun(ctx, F, &ta->nh_act);
  }

  if (acts->fcta[DP_SET_SNAT].ca.act_type == DP_SET_SNAT) {
    LL_FC_PRINTK("[FCH4] snat-act\n");
    ta = &acts->fcta[DP_SET_SNAT];
    dp_pipe_set_nat(ctx, F, &ta->nat_act, 1);
  } else if (acts->fcta[DP_SET_DNAT].ca.act_type == DP_SET_DNAT) {
    LL_FC_PRINTK("[FCH4] dnat-act\n");
    ta = &acts->fcta[DP_SET_DNAT];
    dp_pipe_set_nat(ctx, F, &ta->nat_act, 0);
  }

  if (acts->fcta[DP_SET_RT_TUN_NH].ca.act_type == DP_SET_RT_TUN_NH) {
    ta = &acts->fcta[DP_SET_RT_TUN_NH];
    LL_FC_PRINTK("[FCH4] tun-nh found\n");
    dp_pipe_set_l22_tun_nh(ctx, F, &ta->nh_act);
  } else if (acts->fcta[DP_SET_L3RT_TUN_NH].ca.act_type == DP_SET_L3RT_TUN_NH) {
    LL_FC_PRINTK("[FCH4] l3-rt-tnh-act\n");
    ta = &acts->fcta[DP_SET_L3RT_TUN_NH];
    dp_pipe_set_l32_tun_nh(ctx, F, &ta->nh_act);
  }

  if (acts->fcta[DP_SET_NEIGH_L2].ca.act_type == DP_SET_NEIGH_L2) {
    LL_FC_PRINTK("[FCH4] l2-rt-nh-act\n");
    ta = &acts->fcta[DP_SET_NEIGH_L2];
    dp_do_rt_l2_nh(ctx, F, &ta->nl2);
  } else if (acts->fcta[DP_SET_NEIGH_VXLAN].ca.act_type 
                          == DP_SET_NEIGH_VXLAN) {
    LL_FC_PRINTK("[FCH4] rt-l2-nh-vxlan-act\n");
    ta = &acts->fcta[DP_SET_NEIGH_VXLAN];
    dp_do_rt_l2_vxlan_nh(ctx, F, &ta->nl2vx); 
  }

  if (acts->fcta[DP_SET_ADD_L2VLAN].ca.act_type == DP_SET_ADD_L2VLAN) {
    LL_FC_PRINTK("[FCH4] new-l2-vlan-act\n");
    ta = &acts->fcta[DP_SET_ADD_L2VLAN];
    dp_set_egr_vlan(ctx, F, ta->l2ov.vlan, ta->l2ov.oport);
  } else if (acts->fcta[DP_SET_RM_L2VLAN].ca.act_type 
                                == DP_SET_RM_L2VLAN) {
    LL_FC_PRINTK("[FCH4] strip-l2-vlan-act\n");
    ta = &acts->fcta[DP_SET_RM_L2VLAN];
    dp_set_egr_vlan(ctx, F, 0, ta->l2ov.oport);
  } else {
    bpf_map_delete_elem(&fc_v4_map, &key);
    return 0;
  }

  /* Catch any conditions which need us to go to CP */
  if (F->pm.tcp_flags & (LLB_TCP_FIN|LLB_TCP_RST)) {
    acts->ca.ftrap = 1;
    return 0;
  }

  F->pm.phit |= LLB_DP_FC_HIT;
  LL_FC_PRINTK("[FCH4] oport %d\n",  F->pm.oport);
  dp_unparse_packet(ctx, F);

  dp_do_map_stats(ctx, F, LL_DP_ACLV4_STATS_MAP, acts->ca.cidx);
  F->pm.oport = acts->ca.oif;

  return ret;
}

static int __always_inline
dp_ing_fc_main(void *ctx, struct xfi *F)
{
  __u32 idx = 1;
  LL_FC_PRINTK("[FCHM] Main--\n");
  if (F->pm.pipe_act == 0) {
    if (dp_do_fcv4_lkup(ctx, F) == 1) {
      if (F->pm.pipe_act == LLB_PIPE_RDR) {
        int oif = F->pm.oport;
        return bpf_redirect(oif, 0);         
      }
    }
  }
  bpf_tail_call(ctx, &pgm_tbl, idx);
  return DP_PASS;
}

