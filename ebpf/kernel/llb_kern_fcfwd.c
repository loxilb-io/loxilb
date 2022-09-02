/*
 *  llb_kern_fc.c: LoxiLB kernel cache based forwarding
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
static int  __always_inline
dp_mk_fcv4_key(struct xfi *xf, struct dp_fcv4_key *key)
{
  memcpy(key->smac, xf->l2m.dl_src, 6);
  memcpy(key->dmac, xf->l2m.dl_dst, 6);
  memcpy(key->in_smac, xf->il2m.dl_src, 6);
  memcpy(key->in_dmac, xf->il2m.dl_dst, 6);

  //key->bd         = xf->pm.bd;
  key->bd         = 0; 
  key->daddr      = xf->l3m.ip.daddr;
  key->saddr      = xf->l3m.ip.saddr;
  key->sport      = xf->l3m.source;
  key->dport      = xf->l3m.dest;
  key->l4proto    = xf->l3m.nw_proto;

  //key->in_port    = xf->pm.iport;
  key->in_port    = 0;
  key->in_daddr   = xf->il3m.ip.daddr;
  key->in_saddr   = xf->il3m.ip.saddr;
  key->in_sport   = xf->il3m.source;
  key->in_dport   = xf->il3m.dest;
  key->in_l4proto = xf->il3m.nw_proto;

  return 0;
}

static int __always_inline
dp_do_fcv4_lkup(void *ctx, struct xfi *xf)
{
  struct dp_fcv4_key key;
  struct dp_fc_tacts *acts;
  struct dp_fc_tact *ta;
  int ret = 1;
  int z = 0;

  dp_mk_fcv4_key(xf, &key);

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

  xf->pm.table_id = LL_DP_FCV4_MAP;
  acts = bpf_map_lookup_elem(&fc_v4_map, &key);
  if (!acts) {
    /* xfck - fcache key table is maintained so that 
     * there is no need to make fcv4 key again in
     * tail-call sections
     */
    bpf_map_update_elem(&xfck, &z, &key, BPF_ANY);
    return 0; 
  }

  /* Check timeout */ 
  if (bpf_ktime_get_ns() - acts->its > FC_V4_DPTO) {
    LL_FC_PRINTK("[FCH4] hto");
    bpf_map_update_elem(&xfck, &z, &key, BPF_ANY);
    bpf_map_delete_elem(&fc_v4_map, &key);
    return 0; 
  }

  LL_FC_PRINTK("[FCH4] key found act-sz %d\n", sizeof(struct dp_fc_tacts));

  if (acts->ca.ftrap)
    return 0; 

  if (acts->fcta[DP_SET_RM_VXLAN].ca.act_type == DP_SET_RM_VXLAN) {
    LL_FC_PRINTK("[FCH4] strip-vxlan-act\n");
    ta = &acts->fcta[DP_SET_RM_VXLAN];
    dp_pipe_set_rm_vx_tun(ctx, xf, &ta->nh_act);
  }

  if (acts->fcta[DP_SET_SNAT].ca.act_type == DP_SET_SNAT) {
    LL_FC_PRINTK("[FCH4] snat-act\n");
    ta = &acts->fcta[DP_SET_SNAT];

    if (ta->nat_act.fr == 1 || ta->nat_act.doct) {
      return 0;
    }

    dp_pipe_set_nat(ctx, xf, &ta->nat_act, 1);
    dp_do_map_stats(ctx, xf, LL_DP_NAT4_STATS_MAP, ta->nat_act.rid);
  } else if (acts->fcta[DP_SET_DNAT].ca.act_type == DP_SET_DNAT) {
    LL_FC_PRINTK("[FCH4] dnat-act\n");
    ta = &acts->fcta[DP_SET_DNAT];

    if (ta->nat_act.fr == 1 || ta->nat_act.doct) {
      return 0;
    }

    dp_pipe_set_nat(ctx, xf, &ta->nat_act, 0);
    dp_do_map_stats(ctx, xf, LL_DP_NAT4_STATS_MAP, ta->nat_act.rid);
  }

  if (acts->fcta[DP_SET_RT_TUN_NH].ca.act_type == DP_SET_RT_TUN_NH) {
    ta = &acts->fcta[DP_SET_RT_TUN_NH];
    LL_FC_PRINTK("[FCH4] tun-nh found\n");
    dp_pipe_set_l22_tun_nh(ctx, xf, &ta->nh_act);
  } else if (acts->fcta[DP_SET_L3RT_TUN_NH].ca.act_type == DP_SET_L3RT_TUN_NH) {
    LL_FC_PRINTK("[FCH4] l3-rt-tnh-act\n");
    ta = &acts->fcta[DP_SET_L3RT_TUN_NH];
    dp_pipe_set_l32_tun_nh(ctx, xf, &ta->nh_act);
  }

  if (acts->fcta[DP_SET_NEIGH_L2].ca.act_type == DP_SET_NEIGH_L2) {
    LL_FC_PRINTK("[FCH4] l2-rt-nh-act\n");
    ta = &acts->fcta[DP_SET_NEIGH_L2];
    dp_do_rt_l2_nh(ctx, xf, &ta->nl2);
  }
  if (acts->fcta[DP_SET_NEIGH_VXLAN].ca.act_type == DP_SET_NEIGH_VXLAN) {
    LL_FC_PRINTK("[FCH4] rt-l2-nh-vxlan-act\n");
    ta = &acts->fcta[DP_SET_NEIGH_VXLAN];
    dp_do_rt_l2_vxlan_nh(ctx, xf, &ta->nl2vx); 
  }

  if (acts->fcta[DP_SET_ADD_L2VLAN].ca.act_type == DP_SET_ADD_L2VLAN) {
    LL_FC_PRINTK("[FCH4] new-l2-vlan-act\n");
    ta = &acts->fcta[DP_SET_ADD_L2VLAN];
    dp_set_egr_vlan(ctx, xf, ta->l2ov.vlan, ta->l2ov.oport);
  } else if (acts->fcta[DP_SET_RM_L2VLAN].ca.act_type == DP_SET_RM_L2VLAN) {
    LL_FC_PRINTK("[FCH4] strip-l2-vlan-act\n");
    ta = &acts->fcta[DP_SET_RM_L2VLAN];
    dp_set_egr_vlan(ctx, xf, 0, ta->l2ov.oport);
  } else {
    bpf_map_delete_elem(&fc_v4_map, &key);
    return 0;
  }

  /* Catch any conditions which need us to go to cp/ct */
  if (xf->pm.l4fin) {
    acts->ca.ftrap = 1;
    return 0;
  }

  xf->pm.phit |= LLB_DP_FC_HIT;
  LL_FC_PRINTK("[FCH4] oport %d\n",  xf->pm.oport);
  dp_unparse_packet_always(ctx, xf);
  dp_unparse_packet(ctx, xf);

  dp_do_map_stats(ctx, xf, LL_DP_ACLV4_STATS_MAP, acts->ca.cidx);
  xf->pm.oport = acts->ca.oif;

  return ret;
}

static int __always_inline
dp_ing_fc_main(void *ctx, struct xfi *xf)
{
  __u32 idx = LLB_DP_PKT_SLOW_PGM_ID;
  LL_FC_PRINTK("[FCHM] Main--\n");
  if (xf->pm.pipe_act == 0) {
    if (dp_do_fcv4_lkup(ctx, xf) == 1) {
      if (xf->pm.pipe_act == LLB_PIPE_RDR) {
        int oif = xf->pm.oport;
        return bpf_redirect(oif, 0);         
      }
    }
  }
  bpf_tail_call(ctx, &pgm_tbl, idx);
  return DP_PASS;
}

