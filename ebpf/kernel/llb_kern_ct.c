/*
 *  llb_kern_ct.c: Loxilb kernel eBPF ConnTracking Implementation
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
#if 0
static int
dp_ct_ins2_acl(struct dp_ctv4_key *key,
               struct dp_ctv4_key *xkey,
               struct dp_aclv4_tact *atdat,
               struct dp_aclv4_tact *axtdat)
{
  struct dp_ctv4_dat *xtdat = &axtdat->ctd;
  //struct dp_nat_act *na;
  int ret;

  if (xtdat->xi.nat_flags) {
    atdat->ca.act_type = xtdat->xi.nat_flags & LLB_NAT_DST ?
                             DP_SET_DNAT: DP_SET_SNAT;
    atdat->ca.cidx = xtdat->rid;

  } else {
    atdat->ca.act_type = xtdat->smr != CT_SMR_EST ?
                            DP_SET_DO_CT:DP_SET_NOP; 
    atdat->ca.ftrap = 0;  
    atdat->ca.oif = 0;    
    atdat->ca.cidx = xtdat->rid;
  }

  ret = bpf_map_update_elem(&acl_v4_map, xkey, atdat, BPF_ANY);
  if (ret != 0) return -1;

  return 0;
}
#endif

static int 
dp_ct_proto_xfk_init(struct dp_ctv4_key *key,
                     nxfrm_inf_t *xi,
                     struct dp_ctv4_key *xkey,
                     nxfrm_inf_t *xxi)
{
  xkey->daddr = key->saddr;
  xkey->saddr = key->daddr; 
  xkey->sport = key->dport; 
  xkey->dport = key->sport;
  xkey->l4proto = key->l4proto;
  xkey->zone = key->zone;
  xkey->r = 0;

  /* Apply NAT xfrm if needed */
  if (xi->nat_flags & LLB_NAT_DST) {
    xkey->saddr = xi->nat_xip;
    if (key->l4proto != IPPROTO_ICMP) {
        if (xi->nat_xport)
          xkey->sport = xi->nat_xport;
        else
          xi->nat_xport = key->dport;
    }

    xxi->nat_flags = LLB_NAT_SRC;
    xxi->nat_xip = key->daddr;
    if (key->l4proto != IPPROTO_ICMP)
      xxi->nat_xport = key->dport;
  }
  if (xi->nat_flags & LLB_NAT_SRC) {
    xkey->daddr = xi->nat_xip;

    if (key->l4proto != IPPROTO_ICMP) {
      if (xi->nat_xport)
        xkey->dport = xi->nat_xport;
      else
        xi->nat_xport = key->sport;
    }

    xxi->nat_flags = LLB_NAT_DST;
    xxi->nat_xip = key->saddr;

    if (key->l4proto != IPPROTO_ICMP)
      xxi->nat_xport = key->sport;
  }
  if (xi->nat_flags & LLB_NAT_HDST) {
    xkey->saddr = key->saddr;
    xkey->daddr = key->daddr;

    if (key->l4proto != IPPROTO_ICMP) {
      if (xi->nat_xport)
        xkey->sport = xi->nat_xport;
      else
        xi->nat_xport = key->dport;
    }

    xxi->nat_flags = LLB_NAT_HSRC;
    xxi->nat_xip = 0;
    xi->nat_xip = 0;
    if (key->l4proto != IPPROTO_ICMP)
      xxi->nat_xport = key->dport;
  }
  if (xi->nat_flags & LLB_NAT_HSRC) {
    xkey->saddr = key->saddr;
    xkey->daddr = key->daddr;

    if (key->l4proto != IPPROTO_ICMP) {
      if (xi->nat_xport)
        xkey->dport = xi->nat_xport;
      else
        xi->nat_xport = key->sport;
    }

    xxi->nat_flags = LLB_NAT_HDST;
    xxi->nat_xip = 0;
    xi->nat_xip = 0;
    if (key->l4proto != IPPROTO_ICMP)
      xxi->nat_xport = key->sport;
  }

  return 0;  
}

static int __always_inline
dp_ct3_sm(struct dp_ctv4_dat *tdat,
          struct dp_ctv4_dat *xtdat,
          ct_dir_t dir)
{
  ct_state_t new_state = tdat->pi.l3i.state;
  switch (tdat->pi.l3i.state) {
  case CT_STATE_NONE:
    if (dir == CT_DIR_IN)  {
      new_state = CT_STATE_REQ;
    } else {
      return -1;
    }
    break;
  case CT_STATE_REQ:
    if (dir == CT_DIR_OUT)  {
      new_state = CT_STATE_REP;
    }
    break;
  case CT_STATE_REP:
    if (dir == CT_DIR_IN)  {
      new_state = CT_STATE_EST;
    } 
    break;
  default:
    break;
  }

  tdat->pi.l3i.state = new_state;

  if (new_state == CT_STATE_EST) {
    return 1;
  }

  return 0;
}

static int __always_inline
dp_ct_tcp_sm(void *ctx, struct xfi *F, 
             struct dp_aclv4_tact *atdat,
             struct dp_aclv4_tact *axtdat,
             ct_dir_t dir)
{
  struct dp_ctv4_dat *tdat = &atdat->ctd;
  struct dp_ctv4_dat *xtdat = &axtdat->ctd;
  ct_tcp_pinf_t *ts = &tdat->pi.t;
  ct_tcp_pinf_t *rts = &xtdat->pi.t;
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct tcphdr *t = DP_ADD_PTR(DP_PDATA(ctx), F->pm.l4_off);
  uint8_t tcp_flags = F->pm.tcp_flags;
  ct_tcp_pinfd_t *td = &ts->tcp_cts[dir];
  ct_tcp_pinfd_t *rtd;
  uint32_t seq;
  uint32_t ack;
  uint32_t nstate = 0;

  if (t + 1 > dend) {
    LLBS_PPLN_DROP(F);
    return -1;
  }

  seq = bpf_ntohl(t->seq);
  ack = bpf_ntohl(t->ack_seq);

  bpf_spin_lock(&atdat->lock);

  if (dir == CT_DIR_IN) {
    tdat->pb.bytes += F->pm.l3_len;
    tdat->pb.packets += 1;
  } else {
    xtdat->pb.bytes += F->pm.l3_len;
    xtdat->pb.packets += 1;
  }

  rtd = &ts->tcp_cts[dir == CT_DIR_IN ? CT_DIR_OUT:CT_DIR_IN];

  if (tcp_flags & LLB_TCP_RST) {
    nstate = CT_TCP_CW;
    goto end;
  }

  switch (ts->state) {
  case CT_TCP_CLOSED:

    /* If DP starts after TCP was established
     * we need to somehow handle this particular case
     */
    if (tcp_flags & LLB_TCP_ACK)  {
      td->seq = seq;
      if (td->init_acks) {
        if (ack  > rtd->seq + 2) {
          nstate = CT_TCP_ERR;
          goto end;
        }
      }
      td->init_acks++;
      if (td->init_acks >= CT_TCP_INIT_ACK_THRESHOLD &&
          rtd->init_acks >= CT_TCP_INIT_ACK_THRESHOLD) {
        nstate = CT_TCP_EST;
        break;
      }
    }
    

    if ((tcp_flags & LLB_TCP_SYN) != LLB_TCP_SYN) {
      nstate = CT_TCP_ERR;
      goto end;
    }

    /* SYN sent with ack 0 */
    if (ack != 0 && dir != CT_DIR_IN) {
      nstate = CT_TCP_ERR;
      goto end;
    }

    td->seq = seq;
    nstate = CT_TCP_SS;
    break;
  case CT_TCP_SS:
    if (dir != CT_DIR_OUT) {
      nstate = CT_TCP_ERR;
      goto end;
    }
  
    if ((tcp_flags & (LLB_TCP_SYN|LLB_TCP_ACK)) !=
         (LLB_TCP_SYN|LLB_TCP_ACK)) {
      nstate = CT_TCP_ERR;
      goto end;
    }
  
    if (ack  != rtd->seq + 1) {
      nstate = CT_TCP_ERR;
      goto end;
    }

    td->seq = seq;
    nstate = CT_TCP_SA;
    break;

  case CT_TCP_SA:
    if (dir != CT_DIR_IN) {
      nstate = CT_TCP_ERR;
      goto end;
    } 
  
    if ((tcp_flags & LLB_TCP_ACK) != LLB_TCP_ACK) {
      nstate = CT_TCP_ERR;
      goto end;
    }

    if (ack  != rtd->seq + 1) {
      nstate = CT_TCP_ERR;
      goto end;
    }

    td->seq = seq;
    nstate = CT_TCP_EST;
    break;

  case CT_TCP_EST:
    if (tcp_flags & LLB_TCP_FIN) {
      ts->fndir = dir;
      nstate = CT_TCP_FINI;
      td->seq = seq;
    } else {
      nstate = CT_TCP_EST;
    }
    break;

  case CT_TCP_FINI:
    if (ts->fndir != dir) {
      if ((tcp_flags & (LLB_TCP_FIN|LLB_TCP_ACK)) == 
          (LLB_TCP_FIN|LLB_TCP_ACK)) {
        if (ack  != rtd->seq + 1) {
          nstate = CT_TCP_ERR;
          goto end;
        }

        nstate = CT_TCP_FINI3;
        td->seq = seq;
      } else if (tcp_flags & LLB_TCP_ACK) {
        if (ack  != rtd->seq + 1) {
          nstate = CT_TCP_ERR;
          goto end;
        }
        nstate = CT_TCP_FINI2;
        td->seq = seq;
      }
    }
    break;

  case CT_TCP_FINI2:
    if (ts->fndir != dir) {
      if (tcp_flags & LLB_TCP_FIN) {
        nstate = CT_TCP_FINI3;
        td->seq = seq;
      }
    }
    break;

  case CT_TCP_FINI3:
    if (ts->fndir == dir) {
      if (tcp_flags & LLB_TCP_ACK) {

        if (ack != rtd->seq + 1) {
          nstate = CT_TCP_ERR;
          goto end;
        }
        nstate = CT_TCP_CW;
      }
    }
    break;

  default:
    break;
  }

end:
  ts->state = nstate;
  rts->state = nstate;

  bpf_spin_unlock(&atdat->lock);

  if (nstate == CT_TCP_EST) {
    return CT_SMR_EST;
  } else if (nstate & CT_TCP_CW) {
    return CT_SMR_CTD;
  } else if (nstate & CT_TCP_ERR) {
    return CT_SMR_ERR;
  } else if (nstate & CT_TCP_FIN_MASK) {
    return CT_SMR_FIN;
  }

  return CT_SMR_INPROG;
}

static int __always_inline
dp_ct_udp_sm(void *ctx, struct xfi *F,
             struct dp_aclv4_tact *atdat,
             struct dp_aclv4_tact *axtdat,
             ct_dir_t dir)
{
  struct dp_ctv4_dat *tdat = &atdat->ctd;
  struct dp_ctv4_dat *xtdat = &axtdat->ctd;
  ct_udp_pinf_t *us = &tdat->pi.u;
  ct_udp_pinf_t *xus = &xtdat->pi.u;
  uint32_t nstate = us->state;

  bpf_spin_lock(&atdat->lock);

  if (dir == CT_DIR_IN) {
    tdat->pb.bytes += F->pm.l3_len;
    tdat->pb.packets += 1;
    us->pkts_seen++;
  } else {
    xtdat->pb.bytes += F->pm.l3_len;
    xtdat->pb.packets += 1;
    us->rpkts_seen++;
  }

  switch (us->state) {
  case CT_UDP_CNI:

    if (us->pkts_seen && us->rpkts_seen) {
      nstate = CT_UDP_EST;
    }
    else if (us->pkts_seen > CT_UDP_CONN_THRESHOLD)
      nstate = CT_UDP_UEST;
    break;
  case CT_UDP_UEST:
    if (us->rpkts_seen)
      nstate = CT_UDP_EST;
    break;
  case CT_UDP_EST:
    break;
  default:
    break;
  }


  us->state = nstate;
  xus->state = nstate;

  bpf_spin_unlock(&atdat->lock);

  if (nstate == CT_UDP_UEST)
    return CT_SMR_UEST;
  else if (nstate == CT_UDP_EST)
    return CT_SMR_EST;


  return CT_SMR_INPROG;
}

static int __always_inline
dp_ct_icmp_sm(void *ctx, struct xfi *F, 
              struct dp_aclv4_tact *atdat,
              struct dp_aclv4_tact *axtdat,
              ct_dir_t dir)
{
  struct dp_ctv4_dat *tdat = &atdat->ctd;
  struct dp_ctv4_dat *xtdat = &axtdat->ctd;
  ct_icmp_pinf_t *is = &tdat->pi.i;
  ct_icmp_pinf_t *xis = &xtdat->pi.i;
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct icmphdr *i = DP_ADD_PTR(DP_PDATA(ctx), F->pm.l4_off);
  uint32_t nstate;
  uint16_t seq;

  if (i + 1 > dend) {
    LLBS_PPLN_DROP(F);
    return -1;
  }

  /* We fetch the sequence number even if icmp may not be
   * echo type because we can't call another fn holding
   * spinlock
   */
  seq = bpf_ntohs(i->un.echo.sequence);

  bpf_spin_lock(&atdat->lock);

  if (dir == CT_DIR_IN) {
    tdat->pb.bytes += F->pm.l3_len;
    tdat->pb.packets += 1;
  } else {
    xtdat->pb.bytes += F->pm.l3_len;
    xtdat->pb.packets += 1;
  }

  nstate = is->state;

  switch (i->type) {
  case ICMP_DEST_UNREACH:
    is->state |= CT_ICMP_DUNR;
    goto end;
  case ICMP_TIME_EXCEEDED:
    is->state |= CT_ICMP_TTL;
    goto end;
  case ICMP_REDIRECT:
    is->state |= CT_ICMP_RDR;
    goto end;
  case ICMP_ECHOREPLY:
  case ICMP_ECHO:
    /* Further state-machine processing */
    break;
  default:
    is->state |= CT_ICMP_UNK;
    goto end;
  } 

  switch (is->state) { 
  case CT_ICMP_CLOSED: 
    if (i->type != ICMP_ECHO) { 
      is->errs = 1;
      goto end;
    }
    nstate = CT_ICMP_REQS;
    is->lseq = seq;
    break;
  case CT_ICMP_REQS:
    if (i->type == ICMP_ECHO) {
      is->lseq = seq;
    } else if (i->type == ICMP_ECHOREPLY) {
      if (is->lseq != seq) {
        is->errs = 1;
        goto end;
      }
      nstate = CT_ICMP_REPS;
      is->lseq = seq;
    }
    break;
  case CT_ICMP_REPS:
    /* Connection is tracked now */
  default:
    break;
  }

end:
  is->state = nstate;
  xis->state = nstate;

  bpf_spin_unlock(&atdat->lock);

  if (nstate == CT_ICMP_REPS)
    return CT_SMR_EST;

  return CT_SMR_INPROG;
}

static int __always_inline
dp_ct_sctp_sm(void *ctx, struct xfi *F, 
              struct dp_aclv4_tact *atdat,
              struct dp_aclv4_tact *axtdat,
              ct_dir_t dir)
{
  struct dp_ctv4_dat *tdat = &atdat->ctd;
  struct dp_ctv4_dat *xtdat = &axtdat->ctd;
  ct_sctp_pinf_t *ss = &tdat->pi.s;
  ct_sctp_pinf_t *xss = &xtdat->pi.s;
  uint32_t nstate = 0;
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct sctphdr *s = DP_ADD_PTR(DP_PDATA(ctx), F->pm.l4_off);
  struct sctp_dch *c;
  struct sctp_init_ch *ic;
  struct sctp_cookie *ck;

  bpf_printk("SCTP conntrack");

  if (s + 1 > dend) {
    LLBS_PPLN_DROP(F);
    return -1;
  }

  c = DP_TC_PTR(DP_ADD_PTR(s, sizeof(*s)));
  
  if (c + 1 > dend) {
    LLBS_PPLN_DROP(F);
    return -1;
  }

  nstate = ss->state;
  //bpf_spin_lock(&atdat->lock);

  switch (ss->state) {
  case CT_SCTP_CLOSED:
    if (c->type != SCTP_INIT_CHUNK) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    ic = DP_TC_PTR(DP_ADD_PTR(c, sizeof(*c)));
    if (ic + 1 > dend) {
      LLBS_PPLN_DROP(F);
      goto end;
    }

    ss->itag = ic->tag;

    bpf_printk("Ack %u Tag %u", bpf_htonl(s->vtag), bpf_ntohl(ss->itag));
    nstate = CT_SCTP_INIT;
    break;
  case CT_SCTP_INIT:
    if (c->type != SCTP_INIT_CHUNK_ACK) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    ic = DP_TC_PTR(DP_ADD_PTR(c, sizeof(*c)));
    if (ic + 1 > dend) {
      LLBS_PPLN_DROP(F);
      goto end;
    }

    if (s->vtag != ss->itag) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    ss->otag = ic->tag;

    bpf_printk("Ack %u Tag %u", bpf_htonl(s->vtag), bpf_ntohl(ss->otag));
    nstate = CT_SCTP_INITA;
    break;
  case CT_SCTP_INITA:
    if (c->type != SCTP_COOKIE_ECHO) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    ck = DP_TC_PTR(DP_ADD_PTR(c, sizeof(*c)));
    if (ck + 1 > dend) {
      LLBS_PPLN_DROP(F);
      goto end;
    }

    if (ss->otag != s->vtag) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    ss->cookie = ck->cookie;
    bpf_printk("Ack %u cookie %u", bpf_htonl(s->vtag), bpf_ntohl(ck->cookie));
    nstate = CT_SCTP_COOKIE;
    break;
  case CT_SCTP_COOKIE:
    if (c->type != SCTP_COOKIE_ACK) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    if (ss->itag != s->vtag) {
      nstate = CT_SCTP_ERR;
      goto end;
    }

    bpf_printk("cookie ACK %u", bpf_htonl(s->vtag));
    nstate = CT_SCTP_COOKIEA;
    break;
  default:
    break;
  }
end:
  ss->state = nstate;
  xss->state = nstate;

  //bpf_spin_lock(&atdat->lock);

  if (nstate == CT_SCTP_COOKIEA) {
    return CT_SMR_EST;
  } else if (nstate & CT_SCTP_SHUTC) {
    return CT_SMR_CTD;
  } else if (nstate & CT_SCTP_ERR) {
    return CT_SMR_ERR;
  } else if (nstate & CT_SCTP_FIN_MASK) {
    return CT_SMR_FIN;
  }

  return CT_SMR_INPROG;
}

static int
dp_ct_sm(void *ctx, struct xfi *F,
         struct dp_aclv4_tact *atdat,
         struct dp_aclv4_tact *axtdat,
         ct_dir_t dir)
{
  int sm_ret = 0;

  if (F->pm.l4_off == 0) {
    atdat->ctd.pi.frag = 1;
    return CT_SMR_UNT;
  }

  atdat->ctd.pi.frag = 0;

  switch (F->l3m.nw_proto) {
  case IPPROTO_TCP:
    sm_ret = dp_ct_tcp_sm(ctx, F, atdat, axtdat, dir);
    break;
  case IPPROTO_UDP:
    sm_ret = dp_ct_udp_sm(ctx, F, atdat, axtdat, dir);
    break;
  case IPPROTO_ICMP:
    sm_ret = dp_ct_icmp_sm(ctx, F, atdat, axtdat, dir);
    break;
  case IPPROTO_SCTP:
    sm_ret = dp_ct_sctp_sm(ctx, F, atdat, axtdat, dir);
    break;
  default:
    sm_ret = CT_SMR_UNT;
    break;
  }

  return sm_ret;
}

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         int);
        __type(value,       struct dp_aclv4_tact);
        __uint(max_entries, 2);
} xctk SEC(".maps");

static int __always_inline
dp_ctv4_in(void *ctx, struct xfi *F)
{
  struct dp_ctv4_key key;
  struct dp_ctv4_key xkey;
  struct dp_aclv4_tact *adat;
  struct dp_aclv4_tact *axdat;
  struct dp_aclv4_tact *atdat;
  struct dp_aclv4_tact *axtdat;
  nxfrm_inf_t *xi;
  nxfrm_inf_t *xxi;
  ct_dir_t cdir = CT_DIR_IN;
  int smr = CT_SMR_ERR;
  int k;

  k = 0;
  adat = bpf_map_lookup_elem(&xctk, &k);

  k = 1;
  axdat = bpf_map_lookup_elem(&xctk, &k);

  if (adat == NULL || axdat == NULL) {
    return smr;
  }

  xi = &adat->ctd.xi;
  xxi = &axdat->ctd.xi;
 
  /* CT Key */
  key.daddr = F->l3m.ip.daddr;
  key.saddr = F->l3m.ip.saddr;
  key.sport = F->l3m.source;
  key.dport = F->l3m.dest;
  key.l4proto = F->l3m.nw_proto;
  key.zone = F->pm.zone;
  key.r = 0;

  if (key.l4proto != IPPROTO_TCP &&
      key.l4proto != IPPROTO_UDP &&
      key.l4proto != IPPROTO_ICMP &&
      key.l4proto != IPPROTO_SCTP) {
    return 0;
  }

  xi->nat_flags = F->pm.nf;
  xi->nat_xip   = F->l4m.nxip;
  xi->nat_xport = F->l4m.nxport;

  xxi->nat_flags = 0;
  xxi->nat_xip = 0;
  xxi->nat_xport = 0;

  if (F->pm.nf & (LLB_NAT_DST|LLB_NAT_SRC)) {
    if (xi->nat_xip == 0) {
      if (F->pm.nf == LLB_NAT_DST) {
        xi->nat_flags = LLB_NAT_HDST;
      } else if (F->pm.nf == LLB_NAT_SRC){
        xi->nat_flags = LLB_NAT_HSRC;
      }
    }
  }

  dp_ct_proto_xfk_init(&key, xi, &xkey, xxi);

  atdat = bpf_map_lookup_elem(&acl_v4_map, &key);
  axtdat = bpf_map_lookup_elem(&acl_v4_map, &xkey);
  if (atdat == NULL || axtdat == NULL) {

    bpf_printk("new-ct4");
    adat->ca.ftrap = 0;
    adat->ca.oif = 0;
    adat->ca.cidx = F->pm.rule_id;
    memset(&adat->ctd.pi, 0, sizeof(ct_pinf_t));
    if (xi->nat_flags) {
      adat->ca.act_type = xi->nat_flags & (LLB_NAT_DST|LLB_NAT_HDST) ?
                             DP_SET_DNAT: DP_SET_SNAT;
      adat->nat_act.xip = xi->nat_xip;
      adat->nat_act.xport = xi->nat_xport;
      adat->nat_act.doct = 1;
    } else {
      adat->ca.act_type = DP_SET_DO_CT;
    }
    adat->ctd.dir = cdir;
    adat->ctd.rid = F->pm.rule_id;
    adat->ctd.aid = F->l4m.sel_aid;
    adat->ctd.smr = CT_SMR_INIT;
    bpf_map_update_elem(&acl_v4_map, &key, adat, BPF_ANY);

    axdat->ca.ftrap = 0;
    axdat->ca.oif = 0;
    axdat->ca.cidx = F->pm.rule_id;
    memset(&axdat->ctd.pi, 0, sizeof(ct_pinf_t));
    if (xxi->nat_flags) { 
      axdat->ca.act_type = xxi->nat_flags & (LLB_NAT_DST|LLB_NAT_HDST) ?
                             DP_SET_DNAT: DP_SET_SNAT;
      axdat->nat_act.xip = xxi->nat_xip;
      axdat->nat_act.xport = xxi->nat_xport;
      axdat->nat_act.doct = 1;
    } else {
      axdat->ca.act_type = DP_SET_DO_CT;
    }
    axdat->lts = adat->lts;
    axdat->ctd.dir = CT_DIR_OUT;
    axdat->ctd.smr = CT_SMR_INIT;
    axdat->ctd.rid = adat->ctd.rid;
    axdat->ctd.aid = adat->ctd.aid;
    bpf_map_update_elem(&acl_v4_map, &xkey, axdat, BPF_ANY);

    atdat = bpf_map_lookup_elem(&acl_v4_map, &key);
    axtdat = bpf_map_lookup_elem(&acl_v4_map, &xkey);

  }

  if (atdat != NULL && axtdat != NULL) {
    atdat->lts = bpf_ktime_get_ns();
    axtdat->lts = atdat->lts;
    if (atdat->ctd.dir == CT_DIR_IN) {
      bpf_printk("in-dir");
      smr = dp_ct_sm(ctx, F, atdat, axtdat, CT_DIR_IN);
    } else {
      bpf_printk("out-dir");
      smr = dp_ct_sm(ctx, F, axtdat, atdat, CT_DIR_OUT);
    }

    bpf_printk("smr %d", smr);

    if (smr == CT_SMR_EST) {
      bpf_printk("est");
      if (xi->nat_flags) {
        atdat->nat_act.doct = 0;
        axtdat->nat_act.doct = 0;
      } else {
        atdat->ca.act_type = DP_SET_NOP;
        axtdat->ca.act_type = DP_SET_NOP;
      }
    } else if (smr == CT_SMR_ERR) {
      atdat->ca.act_type = DP_SET_TOCP;
      axtdat->ca.act_type = DP_SET_TOCP;
    }
  }

  return smr; 
}
