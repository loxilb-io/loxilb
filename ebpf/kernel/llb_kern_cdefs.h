/*
 *  llb_dp_cdefs.h: Loxilb eBPF/XDP utility functions 
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 * SPDX-License-Identifier: GPL-2.0
 */
#ifndef __LLB_DP_CDEFS_H__
#define __LLB_DP_CDEFS_H__

#include <linux/bpf.h>
#include <linux/in.h>
#include <linux/if_arp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include "../common/parsing_helpers.h"
#include "../common/llb_dp_mdi.h"
#include "../common/llb_dpapi.h"

#ifndef __stringify
# define __stringify(X)   #X
#endif

#ifndef __section
# define __section(NAME)            \
  __attribute__((section(NAME), used))
#endif

#ifndef __section_tail
# define __section_tail(ID, KEY)          \
  __section(__stringify(ID) "/" __stringify(KEY))
#endif

#define PGM_ENT0    0
#define PGM_ENT1    1

#define SAMPLE_SIZE 64ul
#define MAX_CPUS    128

#ifndef lock_xadd
#define lock_xadd(ptr, val)              \
   ((void)__sync_fetch_and_add(ptr, val))
#endif

struct ll_xmdpi
{
  __u16 iport;
  __u16 oport;
  __u32 skip;
};

struct ll_xmdi {
  union {
      __u64 xmd;
    struct ll_xmdpi pi;
  };
} __attribute__((aligned(4)));

#ifdef HAVE_LEGACY_BPF_MAPS

struct bpf_map_def SEC("maps") intf_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct intf_key),
  .value_size = sizeof(struct dp_intf_tact),
  .max_entries = LLB_INTF_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") intf_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Index xdp_ifidx */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_INTERFACES,
};

struct bpf_map_def SEC("maps") bd_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Index bd_id */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_INTF_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") pkt_ring = {
  .type = BPF_MAP_TYPE_PERF_EVENT_ARRAY,
  .key_size = sizeof(int),
  .value_size = sizeof(__u32),
  .max_entries = MAX_CPUS,
};

struct bpf_map_def SEC("maps") pkts = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Index xdp_ifidx */
  .value_size = sizeof(struct ll_dp_pmdi),
  .max_entries = 1,
};

struct bpf_map_def SEC("maps") fcas = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),
  .value_size = sizeof(struct dp_fc_tacts),
  .max_entries = 1,
};

struct bpf_map_def SEC("maps") xfis = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(int),  /* Index CPU idx */
  .value_size = sizeof(struct xfi),
  .max_entries = 1,
};

struct bpf_map_def SEC("maps") tx_intf_map = {
  .type = BPF_MAP_TYPE_DEVMAP,
  .key_size = sizeof(int),
  .value_size = sizeof(int),
  .max_entries = LLB_INTERFACES,
};

struct bpf_map_def SEC("maps") tx_intf_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Index xdp_ifidx */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_INTERFACES,
};

struct bpf_map_def SEC("maps") tx_bd_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Index bd_id */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_INTF_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") smac_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_smac_key),
  .value_size = sizeof(struct dp_smac_tact),
  .max_entries = LLB_SMAC_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") dmac_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_dmac_key),
  .value_size = sizeof(struct dp_dmac_tact),
  .max_entries = LLB_DMAC_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") tmac_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_tmac_key),
  .value_size = sizeof(struct dp_tmac_tact),
  .max_entries = LLB_TMAC_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") tmac_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* tmac index */
  .value_size = sizeof(struct ll_dp_pmdi),
  .max_entries = LLB_TMAC_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") nh_map = {
  .type = BPF_MAP_TYPE_ARRAY,
  .key_size = sizeof(struct dp_nh_key),
  .value_size = sizeof(struct dp_nh_tact),
  .max_entries = LLB_NH_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") acl_v4_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_ctv4_key),
  .value_size = sizeof(struct dp_aclv4_tact),
  .max_entries = LLB_ACLV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") acl_v4_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_ACLV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") acl_v6_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_ACLV6_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") nat_v4_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_natv4_key),
  .value_size = sizeof(struct dp_natv4_tacts),
  .max_entries = LLB_NATV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") nat_v4_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_NATV4_STAT_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") rt_v4_map = {
  .type = BPF_MAP_TYPE_LPM_TRIE,
  .key_size = sizeof(struct dp_rtv4_key),
  .value_size = sizeof(struct dp_rt_tact),
  .map_flags = BPF_F_NO_PREALLOC,
  .max_entries = LLB_RTV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") rt_v4_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_RTV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") rt_v6_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_RTV6_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") mirr_map = {
  .type = BPF_MAP_TYPE_ARRAY,
  .key_size = sizeof(__u32),
  .value_size = sizeof(struct dp_mirr_tact),
  .max_entries = LLB_MIRR_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") ct_v4_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_ctv4_key),
  .value_size = sizeof(struct dp_ctv4_dat),
  .map_flags = BPF_F_NO_PREALLOC,
  .max_entries = LLB_CTV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") sess_v4_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_sess4_key),
  .value_size = sizeof(struct dp_sess_tact),
  .map_flags = BPF_F_NO_PREALLOC,
  .max_entries = LLB_SESS_MAP_ENTRIES 
};

struct bpf_map_def SEC("maps") sess_v4_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_SESS_MAP_ENTRIES 
};

struct bpf_map_def SEC("maps") fc_v4_map = {
  .type = BPF_MAP_TYPE_HASH,
  .key_size = sizeof(struct dp_fcv4_key),
  .value_size = sizeof(struct dp_fc_tacts),
  .map_flags = BPF_F_NO_PREALLOC,
  .max_entries = LLB_FCV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") fc_v4_stats_map = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(__u32),  /* Counter Index */
  .value_size = sizeof(struct dp_pb_stats),
  .max_entries = LLB_FCV4_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") pgm_tbl = {
  .type = BPF_MAP_TYPE_PROG_ARRAY,
  .key_size = sizeof(__u32),
  .value_size = sizeof(__u32),
  .max_entries =  LLB_PGM_MAP_ENTRIES
};

struct bpf_map_def SEC("maps") polx_map = { 
  .type = BPF_MAP_TYPE_ARRAY,
  .key_size = sizeof(__u32),
  .value_size = sizeof(struct dp_pol_tact),
  .max_entries =  LLB_POL_MAP_ENTRIES 
}; 

struct bpf_map_def SEC("maps") xfck = {
  .type = BPF_MAP_TYPE_PERCPU_ARRAY,
  .key_size = sizeof(int),  /* Index CPU idx */
  .value_size = sizeof(struct dp_fcv4_key),
  .max_entries = 1,
};

#else /* New BTF definitions */

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct intf_key);
        __type(value,       struct dp_intf_tact);
        __uint(max_entries, LLB_INTERFACES);
} intf_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_INTERFACES);
} intf_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_INTF_MAP_ENTRIES);
} bd_stats_map SEC(".maps");

/*
struct {
        __uint(type,        BPF_MAP_TYPE_PERF_EVENT_ARRAY);
        __type(key,         int);
        __type(value,       __u32);
        __uint(max_entries, MAX_CPUS);
} pkt_ring SEC(".maps");
*/

struct bpf_map_def SEC("maps") pkt_ring = {
          .type             = BPF_MAP_TYPE_PERF_EVENT_ARRAY,
          .key_size         = sizeof(int),
          .value_size       = sizeof(__u32),
          .max_entries      = MAX_CPUS,
};

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct ll_dp_pmdi);
        __uint(max_entries, 1);
} pkts SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_fc_tacts);
        __uint(max_entries, 1);
} fcas SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         int);
        __type(value,       struct xfi);
        __uint(max_entries, 1);
} xfis SEC(".maps");

/*
struct {
        __uint(type,        BPF_MAP_TYPE_DEVMAP);
        __type(key,         int);
        __type(value,       int);
        __uint(max_entries, LLB_INTERFACES);
} tx_intf_map SEC(".maps");
*/

struct bpf_map_def SEC("maps") tx_intf_map = {
  .type                     = BPF_MAP_TYPE_DEVMAP,
  .key_size                 = sizeof(int),
  .value_size               = sizeof(int),
  .max_entries              = LLB_INTERFACES,
};

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_INTF_MAP_ENTRIES);
} tx_intf_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_INTF_MAP_ENTRIES);
} tx_bd_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_smac_key);
        __type(value,       struct dp_smac_tact);
        __uint(max_entries, LLB_SMAC_MAP_ENTRIES);
} smac_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_dmac_key);
        __type(value,       struct dp_dmac_tact);
        __uint(max_entries, LLB_DMAC_MAP_ENTRIES);
} dmac_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_tmac_key);
        __type(value,       struct dp_tmac_tact);
        __uint(max_entries, LLB_TMAC_MAP_ENTRIES);
} tmac_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_TMAC_MAP_ENTRIES);
} tmac_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_ARRAY);
        __type(key,         struct dp_nh_key);
        __type(value,       struct dp_nh_tact);
        __uint(max_entries, LLB_NH_MAP_ENTRIES);
} nh_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_ctv4_key);
        __type(value,       struct dp_aclv4_tact);
        __uint(max_entries, LLB_ACLV4_MAP_ENTRIES);
} acl_v4_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_ACLV4_MAP_ENTRIES);
} acl_v4_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_natv4_key);
        __type(value,       struct dp_natv4_tacts);
        __uint(max_entries, LLB_NATV4_MAP_ENTRIES);
} nat_v4_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_NATV4_MAP_ENTRIES);
} nat_v4_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_ACLV6_MAP_ENTRIES);
} acl_v6_stats_map SEC(".maps");

/*
struct {
        __uint(type,        BPF_MAP_TYPE_LPM_TRIE);
        __type(key,         struct dp_rtv4_key);
        __type(value,       struct dp_rt_tact);
        __uint(max_entries, LLB_RTV4_MAP_ENTRIES);
} rt_v4_map SEC(".maps");
*/

struct bpf_map_def SEC("maps") rt_v4_map = {
  .type = BPF_MAP_TYPE_LPM_TRIE,
  .key_size = sizeof(struct dp_rtv4_key),
  .value_size = sizeof(struct dp_rt_tact),
  .map_flags = BPF_F_NO_PREALLOC,
  .max_entries = LLB_RTV4_MAP_ENTRIES
};

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_RTV4_MAP_ENTRIES);
} rt_v4_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_RTV6_MAP_ENTRIES);
} rt_v6_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_mirr_tact);
        __uint(max_entries, LLB_MIRR_MAP_ENTRIES);
} mirr_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_ctv4_key);
        __type(value,       struct dp_ctv4_dat);
        __uint(max_entries, LLB_FCV4_MAP_ENTRIES);
} ct_v4_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_sess4_key);
        __type(value,       struct dp_sess_tact);
        __uint(max_entries, LLB_SESS_MAP_ENTRIES);
} sess_v4_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_SESS_MAP_ENTRIES);
} sess_v4_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_HASH);
        __type(key,         struct dp_fcv4_key);
        __type(value,       struct dp_fc_tacts);
        __uint(max_entries, LLB_FCV4_MAP_ENTRIES);
} fc_v4_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pb_stats);
        __uint(max_entries, LLB_FCV4_MAP_ENTRIES);
} fc_v4_stats_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PROG_ARRAY);
        __type(key,         __u32);
        __type(value,       __u32);
        __uint(max_entries, LLB_PGM_MAP_ENTRIES);
} pgm_tbl SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_ARRAY);
        __type(key,         __u32);
        __type(value,       struct dp_pol_tact);
        __uint(max_entries, LLB_POL_MAP_ENTRIES);
} polx_map SEC(".maps");

struct {
        __uint(type,        BPF_MAP_TYPE_PERCPU_ARRAY);
        __type(key,         int);
        __type(value,       struct dp_fcv4_key);
        __uint(max_entries, 1);
} xfck SEC(".maps");

#endif

static void __always_inline
dp_do_map_stats(struct xdp_md *ctx,  
                struct xfi *xf,
                int xtbl,
                int cidx)
{
  struct dp_pb_stats *pb;
  struct dp_pb_stats pb_new;
  void *map = NULL;
  int key = cidx;

  switch (xtbl) {
  case LL_DP_RTV4_STATS_MAP:
    map = &rt_v4_stats_map;
    break;
  case LL_DP_RTV6_STATS_MAP:
    map = &rt_v6_stats_map;
    break;
  case LL_DP_ACLV4_STATS_MAP:
    map = &acl_v4_stats_map;
    break;
  case LL_DP_ACLV6_STATS_MAP:
    map = &acl_v6_stats_map;
    break;
  case LL_DP_INTF_STATS_MAP:
    map = &intf_stats_map;
    break;
  case LL_DP_TX_INTF_STATS_MAP:
    map = &tx_intf_stats_map;
    break;
  case LL_DP_BD_STATS_MAP:
    map = &bd_stats_map;
    break;
  case LL_DP_TX_BD_STATS_MAP:
    map = &tx_bd_stats_map;
    break;
  case LL_DP_TMAC_STATS_MAP:
    map = &tmac_stats_map;
    break;
  case LL_DP_SESS4_STATS_MAP:
    map = &sess_v4_stats_map;
    break;
  case LL_DP_NAT4_STATS_MAP:
    map = &nat_v4_stats_map;
    break;
  default:
    return;
  }

  pb = bpf_map_lookup_elem(map, &key);
  if (pb) {
    pb->bytes += xf->pm.py_bytes;
    pb->packets += 1;
    LL_DBG_PRINTK("[STAT] %d %llu %llu\n", key, pb->bytes, pb->packets);
    return;
  }

  pb_new.bytes =  xf->pm.py_bytes;;
  pb_new.packets = 1;

  bpf_map_update_elem(map, &key, &pb_new, BPF_ANY);

  return;
}

static void __always_inline
dp_ipv4_new_csum(struct iphdr *iph)
{
    __u16 *iph16 = (__u16 *)iph;
    __u32 csum;
    int i;

    iph->check = 0;

#pragma clang loop unroll(full)
    for (i = 0, csum = 0; i < sizeof(*iph) >> 1; i++)
        csum += *iph16++;

    iph->check = ~((csum & 0xffff) + (csum >> 16));
}

#ifdef LL_TC_EBPF
#include <linux/pkt_cls.h>

#define DP_REDIRECT TC_ACT_REDIRECT
#define DP_DROP     TC_ACT_SHOT
#define DP_PASS     TC_ACT_OK

#define DP_NEED_MIRR(md) (((struct __sk_buff *)md)->cb[0] == LLB_MIRR_MARK)
#define DP_GET_MIRR(md) (((struct __sk_buff *)md)->cb[1])
#define DP_CTX_MIRR(md) (((struct __sk_buff *)md)->cb[0] == LLB_MIRR_MARK)
#define DP_IFI(md) (((struct __sk_buff *)md)->ifindex)
#define DP_PDATA(md) (((struct __sk_buff *)md)->data)
#define DP_PDATA_END(md) (((struct __sk_buff *)md)->data_end)
#define DP_MDATA(md) (((struct __sk_buff *)md)->data_meta)

static int __always_inline
dp_pkt_is_l2mcbc(struct xfi *xf, void *md)
{
  struct __sk_buff *b = md;  

  if (b->pkt_type == PACKET_MULTICAST ||
      b->pkt_type == PACKET_BROADCAST) {
    return 1;
  }
  return 0;
}

static int __always_inline
dp_vlan_info(struct xfi *xf, void *md)
{
  struct __sk_buff *b = md;

  if (b->vlan_present) {
    xf->l2m.dl_type = bpf_htons((__u16)(b->vlan_proto));
    xf->l2m.vlan[0] = bpf_htons((__u16)(b->vlan_tci));
    return 1;
  }

  return 0;
}

static int __always_inline
dp_add_l2(void *md, int delta)
{
  return bpf_skb_change_head(md, delta, 0);
}

static int __always_inline
dp_remove_l2(void *md, int delta)
{
  return bpf_skb_adjust_room(md, -delta, BPF_ADJ_ROOM_MAC, 
                        BPF_F_ADJ_ROOM_FIXED_GSO);
}

static int __always_inline
dp_buf_add_room(void *md, int delta, __u64 flags)
{
  return bpf_skb_adjust_room(md, delta, BPF_ADJ_ROOM_MAC,
                            flags);
}

static int __always_inline
dp_buf_delete_room(void *md, int delta, __u64 flags)
{
  return bpf_skb_adjust_room(md, -delta, BPF_ADJ_ROOM_MAC, 
                            flags);
}

static int __always_inline
dp_redirect_port(void *tbl, struct xfi *xf)
{
  int *oif;
  int key = xf->pm.oport;

  oif = bpf_map_lookup_elem(tbl, &key);
  if (!oif) {
    return TC_ACT_SHOT;
  }
  LL_DBG_PRINTK("[REDR] port %d OIF %d\n", key, *oif);
  return bpf_redirect(*oif, 0);
}

static int __always_inline
dp_rewire_port(void *tbl, struct xfi *xf)
{
  int *oif;
  int key = xf->pm.oport;

  oif = bpf_map_lookup_elem(tbl, &key);
  if (!oif) {
    return TC_ACT_SHOT;
  }
  return bpf_redirect(*oif, BPF_F_INGRESS);
}

static int __always_inline
dp_remove_vlan_tag(void *ctx, struct xfi *xf)
{
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct ethhdr *eth;

  bpf_skb_vlan_pop(ctx);
  eth = DP_TC_PTR(DP_PDATA(ctx));
  dend = DP_TC_PTR(DP_PDATA_END(ctx));
  if (eth + 1 > dend) {
    return -1;
  }
  memcpy(eth->h_dest, xf->l2m.dl_dst, 6);
  memcpy(eth->h_source, xf->l2m.dl_src, 6);
  eth->h_proto = xf->l2m.dl_type;
  return 0;
}

static int __always_inline
dp_insert_vlan_tag(void *ctx, struct xfi *xf, __be16 vlan)
{
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct ethhdr *eth;

  bpf_skb_vlan_push(ctx, bpf_ntohs(xf->l2m.dl_type), bpf_ntohs(vlan));
  eth = DP_TC_PTR(DP_PDATA(ctx));
  dend = DP_TC_PTR(DP_PDATA_END(ctx));
  if (eth + 1 > dend) {
    return -1;
  }
  memcpy(eth->h_dest, xf->l2m.dl_dst, 6);
  memcpy(eth->h_source, xf->l2m.dl_src, 6);
  return 0;
}

static int __always_inline
dp_swap_vlan_tag(void *ctx, struct xfi *xf, __be16 vlan)
{
  bpf_skb_vlan_pop(ctx);
  return dp_insert_vlan_tag(ctx, xf, vlan);
}

static int __always_inline
dp_set_tcp_src_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int tcp_csum_off = xf->pm.l4_off + offsetof(struct tcphdr, check);
  int ip_src_off = xf->pm.l3_off + offsetof(struct iphdr, saddr);
  __be32 old_sip = xf->l3m.ip.saddr;  

  bpf_l4_csum_replace(md, tcp_csum_off, old_sip, xip, BPF_F_PSEUDO_HDR |sizeof(xip));
  bpf_l3_csum_replace(md, ip_csum_off, old_sip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_src_off, &xip, sizeof(xip), 0);

  xf->l3m.ip.saddr = xip;  

  return 0;
}

static int __always_inline
dp_set_tcp_dst_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int tcp_csum_off = xf->pm.l4_off + offsetof(struct tcphdr, check);
  int ip_dst_off = xf->pm.l3_off + offsetof(struct iphdr, daddr);
  __be32 old_dip = xf->l3m.ip.daddr;  

  bpf_l4_csum_replace(md, tcp_csum_off, old_dip, xip, BPF_F_PSEUDO_HDR | sizeof(xip));
  bpf_l3_csum_replace(md, ip_csum_off, old_dip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_dst_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.daddr = xip;  

  return 0;
}

static int __always_inline
dp_set_tcp_sport(void *md, struct xfi *xf, __be16 xport)
{
  int tcp_csum_off = xf->pm.l4_off + offsetof(struct tcphdr, check);
  int tcp_sport_off = xf->pm.l4_off + offsetof(struct tcphdr, source);
  __be32 old_sport = xf->l3m.source;

  bpf_l4_csum_replace(md, tcp_csum_off, old_sport, xport, sizeof(xport));
  bpf_skb_store_bytes(md, tcp_sport_off, &xport, sizeof(xport), 0);
  xf->l3m.source = xport;

  return 0;
}

static int __always_inline
dp_set_tcp_dport(void *md, struct xfi *xf, __be16 xport)
{
  int tcp_csum_off = xf->pm.l4_off + offsetof(struct tcphdr, check);
  int tcp_dport_off = xf->pm.l4_off + offsetof(struct tcphdr, dest);
  __be32 old_dport = xf->l3m.dest;

  bpf_l4_csum_replace(md, tcp_csum_off, old_dport, xport, sizeof(xport));
  bpf_skb_store_bytes(md, tcp_dport_off, &xport, sizeof(xport), 0);
  xf->l3m.dest = xport;

  return 0;
}

static int __always_inline
dp_set_udp_src_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int udp_csum_off = xf->pm.l4_off + offsetof(struct udphdr, check);
  int ip_src_off = xf->pm.l3_off + offsetof(struct iphdr, saddr);
  __be16 csum = 0;
  __be32 old_sip = xf->l3m.ip.saddr;  
  
  /* UDP checksum = 0 is valid */
  bpf_skb_store_bytes(md, udp_csum_off, &csum, sizeof(csum), 0);
  bpf_l3_csum_replace(md, ip_csum_off, old_sip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_src_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.saddr = xip;  

  return 0;
}

static int __always_inline
dp_set_udp_dst_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int udp_csum_off = xf->pm.l4_off + offsetof(struct udphdr, check);
  int ip_dst_off = xf->pm.l3_off + offsetof(struct iphdr, daddr);
  __be16 csum = 0;
  __be32 old_dip = xf->l3m.ip.daddr;  
  
  /* UDP checksum = 0 is valid */
  bpf_skb_store_bytes(md, udp_csum_off, &csum, sizeof(csum), 0);
    bpf_l3_csum_replace(md, ip_csum_off, old_dip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_dst_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.daddr = xip;  

  return 0;
}

static int __always_inline
dp_set_udp_sport(void *md, struct xfi *xf, __be16 xport)
{
  int udp_csum_off = xf->pm.l4_off + offsetof(struct udphdr, check);
  int udp_sport_off = xf->pm.l4_off + offsetof(struct udphdr, source);
  __be16 csum = 0;

  /* UDP checksum = 0 is valid */
  bpf_skb_store_bytes(md, udp_csum_off, &csum, sizeof(csum), 0);
  bpf_skb_store_bytes(md, udp_sport_off, &xport, sizeof(xport), 0);
  xf->l3m.source = xport;

  return 0;
}

static int __always_inline
dp_set_udp_dport(void *md, struct xfi *xf, __be16 xport)
{
  int udp_csum_off = xf->pm.l4_off + offsetof(struct udphdr, check);
  int udp_dport_off = xf->pm.l4_off + offsetof(struct udphdr, dest);
  __be16 csum = 0;

  /* UDP checksum = 0 is valid */
  bpf_skb_store_bytes(md, udp_csum_off, &csum, sizeof(csum), 0);
  bpf_skb_store_bytes(md, udp_dport_off, &xport, sizeof(xport), 0);
  xf->l3m.dest = xport;

  return 0;
}

static int __always_inline
dp_set_icmp_src_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int ip_src_off = xf->pm.l3_off + offsetof(struct iphdr, saddr);
  __be32 old_sip = xf->l3m.ip.saddr;  
  
  bpf_l3_csum_replace(md, ip_csum_off, old_sip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_src_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.saddr = xip;  

  return 0;
}

static int __always_inline
dp_set_icmp_dst_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int ip_dst_off = xf->pm.l3_off + offsetof(struct iphdr, daddr);
  __be32 old_dip = xf->l3m.ip.daddr;  
  
  bpf_l3_csum_replace(md, ip_csum_off, old_dip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_dst_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.daddr = xip;  

  return 0;
}

static int __always_inline
dp_set_sctp_src_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int ip_src_off = xf->pm.l3_off + offsetof(struct iphdr, saddr);
  __be32 old_sip = xf->l3m.ip.saddr;  
  
  bpf_l3_csum_replace(md, ip_csum_off, old_sip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_src_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.saddr = xip;  

  return 0;
}

static int __always_inline
dp_set_sctp_dst_ip(void *md, struct xfi *xf, __be32 xip)
{
  int ip_csum_off  = xf->pm.l3_off + offsetof(struct iphdr, check);
  int ip_dst_off = xf->pm.l3_off + offsetof(struct iphdr, daddr);
  __be32 old_dip = xf->l3m.ip.daddr;  
  
  bpf_l3_csum_replace(md, ip_csum_off, old_dip, xip, sizeof(xip));
  bpf_skb_store_bytes(md, ip_dst_off, &xip, sizeof(xip), 0);
  xf->l3m.ip.daddr = xip;  

  return 0;
}

static int __always_inline
dp_set_sctp_sport(void *md, struct xfi *xf, __be16 xport)
{
  uint32_t csum = 0;
  int sctp_csum_off = xf->pm.l4_off + offsetof(struct sctphdr, checksum);
  int sctp_sport_off = xf->pm.l4_off + offsetof(struct sctphdr, source);

  bpf_skb_store_bytes(md, sctp_csum_off, &csum , sizeof(csum), 0);
  bpf_skb_store_bytes(md, sctp_sport_off, &xport, sizeof(xport), 0);
  xf->l3m.source = xport;

  return 0;
}

static int __always_inline
dp_set_sctp_dport(void *md, struct xfi *xf, __be16 xport)
{
  uint32_t csum = 0;
  int sctp_csum_off = xf->pm.l4_off + offsetof(struct sctphdr, checksum); 
  int sctp_dport_off = xf->pm.l4_off + offsetof(struct sctphdr, dest);

  bpf_skb_store_bytes(md, sctp_csum_off, &csum , sizeof(csum), 0);
  bpf_skb_store_bytes(md, sctp_dport_off, &xport, sizeof(xport), 0);
  xf->l3m.dest = xport;

  return 0;
}

static int __always_inline
dp_do_dnat(void *ctx, struct xfi *xf, __be32 xip, __be16 xport)
{
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  if (xf->l3m.nw_proto == IPPROTO_TCP)  {
    struct tcphdr *tcp = DP_ADD_PTR(DP_PDATA(ctx), xf->pm.l4_off);
    if (tcp + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    if (xip == 0) {
      /* Hairpin nat to host */
      xip = xf->l3m.ip.saddr;
      dp_set_tcp_src_ip(ctx, xf, xf->l3m.ip.daddr);
      dp_set_tcp_dst_ip(ctx, xf, xip);
    } else {
      dp_set_tcp_dst_ip(ctx, xf, xip);
    }
    dp_set_tcp_dport(ctx, xf, xport);
  } else if (xf->l3m.nw_proto == IPPROTO_UDP)  {
    struct udphdr *udp = DP_ADD_PTR(DP_PDATA(ctx), xf->pm.l4_off);

    if (udp + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    if (xip == 0) {
      /* Hairpin nat to host */
      xip = xf->l3m.ip.saddr;
      dp_set_udp_src_ip(ctx, xf, xf->l3m.ip.daddr);
      dp_set_udp_dst_ip(ctx, xf, xip);
    } else {
      dp_set_udp_dst_ip(ctx, xf, xip);
    }
    dp_set_udp_dport(ctx, xf, xport);
  } else if (xf->l3m.nw_proto == IPPROTO_SCTP)  {
    struct sctphdr *sctp = DP_ADD_PTR(DP_PDATA(ctx), xf->pm.l4_off);

    if (sctp + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    if (xip == 0) {
      /* Hairpin nat to host */
      xip = xf->l3m.ip.saddr;
      dp_set_sctp_src_ip(ctx, xf, xf->l3m.ip.daddr);
      dp_set_sctp_dst_ip(ctx, xf, xip);
    } else {
      dp_set_sctp_dst_ip(ctx, xf, xip);
    }
    dp_set_sctp_dport(ctx, xf, xport);
  } else if (xf->l3m.nw_proto == IPPROTO_ICMP)  {
    dp_set_icmp_dst_ip(ctx, xf, xip);
  }

  return 0;
}

static int __always_inline
dp_do_snat(void *ctx, struct xfi *xf, __be32 xip, __be16 xport)
{
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  if (xf->l3m.nw_proto == IPPROTO_TCP)  {
    struct tcphdr *tcp = DP_ADD_PTR(DP_PDATA(ctx), xf->pm.l4_off);
    if (tcp + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    if (xip == 0) {
      /* Hairpin nat to host */
      xip = xf->l3m.ip.saddr;
      dp_set_tcp_src_ip(ctx, xf, xf->l3m.ip.daddr);
      dp_set_tcp_dst_ip(ctx, xf, xip);
    } else {
      dp_set_tcp_src_ip(ctx, xf, xip);
    }
    dp_set_tcp_sport(ctx, xf, xport);
  } else if (xf->l3m.nw_proto == IPPROTO_UDP)  {
    struct udphdr *udp = DP_ADD_PTR(DP_PDATA(ctx), xf->pm.l4_off);

    if (udp + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    if (xip == 0) {
      /* Hairpin nat to host */
      xip = xf->l3m.ip.saddr;
      dp_set_udp_src_ip(ctx, xf, xf->l3m.ip.daddr);
      dp_set_udp_dst_ip(ctx, xf, xip);
    } else {
      dp_set_udp_src_ip(ctx, xf, xip);
    }
    dp_set_udp_sport(ctx, xf, xport);
  } else if (xf->l3m.nw_proto == IPPROTO_SCTP)  {
    struct sctphdr *sctp = DP_ADD_PTR(DP_PDATA(ctx), xf->pm.l4_off);

    if (sctp + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    if (xip == 0) {
      /* Hairpin nat to host */
      xip = xf->l3m.ip.saddr;
      dp_set_sctp_src_ip(ctx, xf, xf->l3m.ip.daddr);
      dp_set_sctp_dst_ip(ctx, xf, xip);
    } else {
      dp_set_sctp_src_ip(ctx, xf, xip);
    }
    dp_set_sctp_sport(ctx, xf, xport);
  } else if (xf->l3m.nw_proto == IPPROTO_ICMP)  {
    dp_set_icmp_src_ip(ctx, xf, xip);
  }

  return 0;
}

static __u32 __always_inline
dp_get_pkt_hash(void *md)
{
  return bpf_get_hash_recalc(md);
}

#else /* XDP utilities */

#define DP_NEED_MIRR(md) (0)
#define DP_GET_MIRR(md)  (0)
#define DP_REDIRECT XDP_REDIRECT
#define DP_DROP     XDP_DROP
#define DP_PASS     XDP_PASS

static int __always_inline
dp_pkt_is_l2mcbc(struct xfi *xf, void *md)
{
  if (xf->l2m.dl_dst[0] & 1) {
    return 1;
  }

  if (xf->l2m.dl_dst[0] == 0xff &&
      xf->l2m.dl_dst[1] == 0xff &&
      xf->l2m.dl_dst[2] == 0xff &&
      xf->l2m.dl_dst[3] == 0xff &&
      xf->l2m.dl_dst[4] == 0xff &&
      xf->l2m.dl_dst[5] == 0xff) {
    return 1;
  }

  return 0;
}

static int __always_inline
dp_add_l2(void *md, int delta)
{
  return bpf_xdp_adjust_head(md, -delta);
}

static int __always_inline
dp_remove_l2(void *md, int delta)
{
  return bpf_xdp_adjust_head(md, delta);
}

static int __always_inline
dp_buf_add_room(void *md, int delta, __u64 flags)
{
  return bpf_xdp_adjust_head(md, -delta);
}

static int __always_inline
dp_buf_delete_room(void *md, int delta, __u64 flags)
{
  return bpf_xdp_adjust_head(md, delta);
}

static int __always_inline
dp_redirect_port(void *tbl, struct xfi *xf)
{
  return bpf_redirect_map(tbl, xf->pm.oport, 0);
}

static int __always_inline
dp_rewire_port(void *tbl, struct xfi *xf)
{
  /* Not supported */
  return 0;
}

#define DP_IFI(md) (((struct xdp_md *)md)->ingress_ifindex)
#define DP_PDATA(md) (((struct xdp_md *)md)->data)
#define DP_PDATA_END(md) (((struct xdp_md *)md)->data_end)
#define DP_MDATA(md) (((struct xdp_md *)md)->data_meta)

static int __always_inline
dp_remove_vlan_tag(void *ctx, struct xfi *xf)
{
  void *start = DP_TC_PTR(DP_PDATA(ctx));
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct ethhdr *eth;
  struct vlan_hdr *vlh;

  if (start + (sizeof(*eth) + sizeof(*vlh)) > dend) {
    return -1;
  }
  eth = DP_ADD_PTR(DP_PDATA(ctx), (int)sizeof(struct vlan_hdr));
  memcpy(eth->h_dest, xf->l2m.dl_dst, 6);
  memcpy(eth->h_source, xf->l2m.dl_src, 6);
  eth->h_proto = xf->l2m.dl_type;
  if (dp_remove_l2(ctx, (int)sizeof(struct vlan_hdr))) {
    return -1;
  }
  return 0;
}

static int __always_inline
dp_insert_vlan_tag(void *ctx, struct xfi *xf, __be16 vlan)
{
  struct ethhdr *neth;
  struct vlan_hdr *vlh;
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  if (dp_add_l2(ctx, (int)sizeof(struct vlan_hdr))) {
    return -1;
  }

  neth = DP_TC_PTR(DP_PDATA(ctx));
  dend = DP_TC_PTR(DP_PDATA_END(ctx));

  /* Revalidate for satisfy eBPF verifier */
  if (DP_TC_PTR(neth) + sizeof(*neth) > dend) {
    return -1;
  }

  memcpy(neth->h_dest, xf->l2m.dl_dst, 6);
  memcpy(neth->h_source, xf->l2m.dl_src, 6);

  /* FIXME : */
  neth->h_proto = bpf_htons(ETH_P_8021Q);

  vlh = DP_ADD_PTR(DP_PDATA(ctx), sizeof(*neth));

  if (DP_TC_PTR(vlh) + sizeof(*vlh) > dend) {
    return -1;
  }

  vlh->h_vlan_TCI = vlan;
  /* FIXME : */
  vlh->h_vlan_encapsulated_proto = xf->l2m.dl_type;
  return 0;
}

static int __always_inline
dp_swap_vlan_tag(void *ctx, struct xfi *xf, __be16 vlan)
{
  struct ethhdr *eth;
  struct vlan_hdr *vlh;
  void *start = DP_TC_PTR(DP_PDATA(ctx));
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));

  if ((start +  sizeof(*eth)) > dend) {
    return -1;
  }
  eth = DP_TC_PTR(DP_PDATA(ctx));
  memcpy(eth->h_dest, xf->l2m.dl_dst, 6);
  memcpy(eth->h_source, xf->l2m.dl_src, 6);

  vlh = DP_ADD_PTR(DP_PDATA(ctx), sizeof(*eth));
  if (DP_TC_PTR(vlh) + sizeof(*vlh) > dend) {
    return -1;
  }
  vlh->h_vlan_TCI = vlan;
  return 0;
}

static int __always_inline
dp_do_snat(void *ctx, struct xfi *xf, __be32 xip, __be16 xport)
{
  /* FIXME - TBD */
  return 0;
}

static int __always_inline
dp_do_dnat(void *ctx, struct xfi *xf, __be32 xip, __be16 xport)
{
  /* FIXME - TBD */
  return 0;
}

static __u32 __always_inline
dp_get_pkt_hash(void *md)
{
  /* FIXME - TODO */
  return 0;
}

#endif  /* End of XDP utilities */

static int __always_inline
dp_do_out_vlan(void *ctx, struct xfi *xf)
{
  void *start = DP_TC_PTR(DP_PDATA(ctx));
  void *dend = DP_TC_PTR(DP_PDATA_END(ctx));
  struct ethhdr *eth;
  int vlan;

  vlan = xf->pm.bd;

  if (vlan == 0) {
    /* Strip existing vlan. Nothing to do if there was no vlan tag */
    if (xf->l2m.vlan[0] != 0) {
      if (dp_remove_vlan_tag(ctx, xf) != 0) {
        LLBS_PPLN_DROP(xf);
        return -1;
      }
    } else {
      if (start + sizeof(*eth) > dend) {
        LLBS_PPLN_DROP(xf);
        return -1;
      }
      eth = DP_TC_PTR(DP_PDATA(ctx));
      memcpy(eth->h_dest, xf->l2m.dl_dst, 6);
      memcpy(eth->h_source, xf->l2m.dl_src, 6);
    }
    return 0;
  } else {
    /* If existing vlan tag was present just replace vlan-id, else 
     * push a new vlan tag and set the vlan-id
     */
    eth = DP_TC_PTR(DP_PDATA(ctx));
    if (xf->l2m.vlan[0] != 0) {
      if (dp_swap_vlan_tag(ctx, xf, vlan) != 0) {
        LLBS_PPLN_DROP(xf);
        return -1;
      }
    } else {
      if (dp_insert_vlan_tag(ctx, xf, vlan) != 0) {
        LLBS_PPLN_DROP(xf);
        return -1;
      }
    }
  }

  return 0;
}

static int __always_inline
dp_pop_outer_l2_metadata(void *md, struct xfi *xf)
{
  memcpy(&xf->l2m.dl_type, &xf->il2m.dl_type, 
         sizeof(xf->l2m) - sizeof(xf->l2m.vlan));

  memcpy(xf->pm.lkup_dmac, xf->il2m.dl_dst, 6);
  xf->il2m.valid = 0;

  return 0;
}

static int __always_inline
dp_pop_outer_metadata(void *md, struct xfi *xf, int l2tun)
{
  /* Reset pipeline metadata */
  memcpy(&xf->l3m, &xf->il3m, sizeof(xf->l3m));

  xf->pm.tcp_flags = xf->pm.itcp_flags;
  xf->pm.l4fin = xf->pm.il4fin;
  xf->pm.l3_off = xf->pm.il3_off;
  xf->pm.l4_off = xf->pm.il4_off;
  xf->il3m.valid = 0;
  xf->tm.tun_decap = 1;

  if (l2tun) {
    return dp_pop_outer_l2_metadata(md, xf);  
  }

  return 0;
}

static int __always_inline
dp_do_strip_vxlan(void *md, struct xfi *xf, int olen)
{
  struct ethhdr *eth;
  struct vlan_hdr *vlh;
  void *dend;

  if (dp_buf_delete_room(md, olen, BPF_F_ADJ_ROOM_FIXED_GSO)  < 0) {
    LL_DBG_PRINTK("Failed MAC remove\n");
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  eth = DP_TC_PTR(DP_PDATA(md));
  dend = DP_TC_PTR(DP_PDATA_END(md));

  if (eth + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }
  memcpy(eth->h_dest, xf->il2m.dl_dst, 2*6);
  if (xf->il2m.vlan[0] != 0) {
    vlh = DP_ADD_PTR(eth, sizeof(*eth));
    if (vlh + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }
    vlh->h_vlan_encapsulated_proto = xf->il2m.dl_type;
  } else {
    eth->h_proto = xf->il2m.dl_type;
  }

#if 0
  /* Reset pipeline metadata */
  memcpy(&xf->l3m, &xf->il3m, sizeof(xf->l3m));
  memcpy(&xf->l2m, &xf->il2m, sizeof(xf->l2m));

  memcpy(xf->pm.lkup_dmac, eth->h_dest, 6);

  xf->il3m.valid = 0;
  xf->il2m.valid = 0;
  xf->tm.tun_decap = 1;
#endif

  return 0;
}

static int __always_inline
dp_do_ins_vxlan(void *md,
                struct xfi *xf,
                __be32 rip,
                __be32 sip,
                __be32 tid,
                int skip_md) 
{
  void *dend;
  struct ethhdr *eth;
  struct ethhdr *ieth;
  struct iphdr *iph;
  struct udphdr *udp;
  struct vxlan_hdr *vx; 
  int olen, l2_len;
  __u64 flags;

  /* We do not pass vlan header inside vxlan */
  if (xf->l2m.vlan[0] != 0) {
    if (dp_remove_vlan_tag(md, xf) < 0) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }
  }

  olen   = sizeof(*iph)  + sizeof(*udp) + sizeof(*vx); 
  l2_len = sizeof(*eth);

    flags = BPF_F_ADJ_ROOM_FIXED_GSO |
          BPF_F_ADJ_ROOM_ENCAP_L3_IPV4 |
          BPF_F_ADJ_ROOM_ENCAP_L4_UDP |
          BPF_F_ADJ_ROOM_ENCAP_L2(l2_len);
    olen += l2_len;

    /* add room between mac and network header */
    if (dp_buf_add_room(md, olen, flags)) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  eth = DP_TC_PTR(DP_PDATA(md));
  dend = DP_TC_PTR(DP_PDATA_END(md));

  if (eth + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

#if 0
  /* 
   * FIXME - Inner ethernet 
   * No need to copy but if we dont 
   * inner eth header is sometimes not set
   * properly especially when incoming packet
   * was vlan tagged
   */
  if (xf->l2m.vlan[0]) {
    memcpy(eth->h_dest, xf->il2m.dl_dst, 2*6);
    eth->h_proto = xf->il2m.dl_type;
  }
#endif

  iph = (void *)(eth + 1);
  if (iph + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  /* Outer IP header */ 
  iph->version  = 4;
  iph->ihl      = 5;
  iph->tot_len  = bpf_htons(xf->pm.l3_len +  olen);
  iph->ttl      = 64; // FIXME - Copy inner
  iph->protocol = IPPROTO_UDP;
  iph->saddr    = sip;
  iph->daddr    = rip;

  dp_ipv4_new_csum((void *)iph);

  udp = (void *)(iph + 1);
  if (udp + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  /* Outer UDP header */
  udp->source = xf->l3m.source + VXLAN_UDP_SPORT;
  udp->dest   = bpf_htons(VXLAN_UDP_DPORT);
  udp->check  = 0;
  udp->len    = bpf_htons(xf->pm.l3_len +  olen - sizeof(*iph));

  /* VxLAN header */
  vx = (void *)(udp + 1);
  if (vx + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  /* Control agent should pass tunnel-id something like this -
   * bpf_htonl(((__le32)(tid) << 8) & 0xffffff00);
   */
  vx->vx_vni   = tid;
  vx->vx_flags = VXLAN_VI_FLAG_ON;

  /* Inner eth header -
   * XXX If we do not copy, inner eth is zero'd out
   */
  ieth = (void *)(vx + 1);
  if (ieth + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  memcpy(ieth->h_dest, xf->il2m.dl_dst, 2*6);
  ieth->h_proto = xf->il2m.dl_type;

  /* Tunnel metadata */
  xf->tm.tun_type  = LLB_TUN_VXLAN;
  xf->tm.tunnel_id = bpf_ntohl(tid);
  xf->pm.tun_off   = sizeof(*eth) + 
                    sizeof(*iph) + 
                    sizeof(*udp);
  xf->tm.tun_encap = 1;

  /* Reset flags essential for L2 header rewrite */
  xf->l2m.vlan[0] = 0;
  xf->l2m.dl_type = bpf_htons(ETH_P_IP);

  if (skip_md) {
    return 0;
  }

  /* 
   * Reset pipeline metadata 
   * If it is called from deparser, there is no need
   * to do the following (set skip_md = 1)
   */
  memcpy(&xf->il3m, &xf->l3m, sizeof(xf->l3m));
  memcpy(&xf->il2m, &xf->l2m, sizeof(xf->l2m));
  xf->il2m.vlan[0] = 0;

  /* Outer L2 - MAC addr are invalid as of now */
  xf->pm.lkup_dmac[0] = 0xff;

  /* Outer L3 */
  xf->l3m.ip.saddr = sip;
  xf->l3m.ip.daddr = rip;
  xf->l3m.source = udp->source;
  xf->l3m.dest = udp->dest;
  xf->pm.l3_off = sizeof(*eth);
  xf->pm.l4_off = sizeof(*eth) + sizeof(*iph);
  
    return 0;
}

static int __always_inline
dp_do_strip_gtp(void *md, struct xfi *xf, int olen)
{
  struct ethhdr *eth;
  void *dend;

  if (olen < sizeof(*eth)) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  if (dp_buf_delete_room(md, olen - sizeof(*eth), BPF_F_ADJ_ROOM_FIXED_GSO)  < 0) {
    LL_DBG_PRINTK("Failed gtph remove\n");
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  eth = DP_TC_PTR(DP_PDATA(md));
  dend = DP_TC_PTR(DP_PDATA_END(md));

  if (eth + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  /* Recreate eth header */
  memcpy(eth->h_dest, xf->l2m.dl_dst, 2*6);
  eth->h_proto = xf->l2m.dl_type;

  /* We do not care about vlan's now
   * After routing it will be set as per outgoing BD
   */
  xf->l2m.vlan[0] = 0;
  xf->l2m.vlan[1] = 0;

#if 0
  /* Reset pipeline metadata */
  memcpy(&xf->l3m, &xf->il3m, sizeof(xf->l3m));
  memcpy(xf->pm.lkup_dmac, eth->h_dest, 6);

  xf->il3m.valid = 0;
  xf->il2m.valid = 0;
  xf->tm.tun_decap = 1;
#endif

  return 0;
}

static int __always_inline
dp_do_ins_gtp(void *md,
              struct xfi *xf,
              __be32 rip,
              __be32 sip,
              __be32 tid,
              __u8 qfi,
              int skip_md) 
{
  void *dend;
  struct gtp_v1_hdr *gh;
  struct gtp_v1_ehdr *geh;
  struct gtp_dl_pdu_sess_hdr *gedh;
  struct ethhdr *eth;
  struct iphdr *iph;
  struct udphdr *udp;
  int olen;
  __u64 flags;
  int ghlen;
  __u8 espn;

  if (qfi) {
    ghlen = sizeof(*gh) + sizeof(*geh) + sizeof(*gedh);
    espn = GTP_EXT_FM;
  } else {
    ghlen = sizeof(*gh);
    espn = 0;
  }

  olen   = sizeof(*iph)  + sizeof(*udp) + ghlen;

  flags = BPF_F_ADJ_ROOM_FIXED_GSO |
          BPF_F_ADJ_ROOM_ENCAP_L3_IPV4 |
          BPF_F_ADJ_ROOM_ENCAP_L4_UDP;

  /* add room between mac and network header */
  if (dp_buf_add_room(md, olen, flags)) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  eth = DP_TC_PTR(DP_PDATA(md));
  dend = DP_TC_PTR(DP_PDATA_END(md));

  if (eth + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  iph = (void *)(eth + 1);
  if (iph + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  /* Outer IP header */ 
  iph->version  = 4;
  iph->ihl      = 5;
  iph->tot_len  = bpf_htons(xf->pm.l3_len +  olen);
  iph->ttl      = 64; // FIXME - Copy inner
  iph->protocol = IPPROTO_UDP;
  iph->saddr    = sip;
  iph->daddr    = rip;

  dp_ipv4_new_csum((void *)iph);

  udp = (void *)(iph + 1);
  if (udp + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  /* Outer UDP header */
  udp->source = bpf_htons(GTPU_UDP_SPORT);
  udp->dest   = bpf_htons(GTPU_UDP_DPORT);
  udp->check  = 0;
  udp->len    = bpf_htons(xf->pm.l3_len +  olen - sizeof(*iph));

  /* GTP header */
  gh = (void *)(udp + 1);
  if (gh + 1 > dend) {
    LLBS_PPLN_DROP(xf);
    return -1;
  }

  gh->ver = GTP_VER_1;
  gh->pt = 1;
  gh->espn = espn;
  gh->teid = tid;
  gh->mt = GTP_MT_TPDU;
  gh->mlen = bpf_ntohs(xf->pm.l3_len + ghlen);
  
  if (qfi) {
    /* GTP extension header */
    geh = (void *)(gh + 1);
    if (geh + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    geh->seq = 0;
    geh->npdu = 0;
    geh->next_hdr = GTP_NH_PDU_SESS;

    gedh = (void *)(geh + 1);
    if (gedh + 1 > dend) {
      LLBS_PPLN_DROP(xf);
      return -1;
    }

    gedh->cmn.len = 1;
    gedh->cmn.pdu_type = GTP_PDU_SESS_DL;
    gedh->qfi = qfi;
    gedh->ppp = 0;
    gedh->rqi = 0;
    gedh->next_hdr = 0;
  }
  /* Tunnel metadata */
  xf->tm.tun_type  = LLB_TUN_GTP;
  xf->tm.tunnel_id = bpf_ntohl(tid);
  xf->pm.tun_off   = sizeof(*eth) + 
                    sizeof(*iph) + 
                    sizeof(*udp);
  xf->tm.tun_encap = 1;

  if (skip_md) {
    return 0;
  }

  /* 
   * Reset pipeline metadata 
   * If it is called from deparser, there is no need
   * to do the following (set skip_md = 1)
   */
  memcpy(&xf->il3m, &xf->l3m, sizeof(xf->l3m));
  xf->il2m.vlan[0] = 0;

  /* Outer L2 - MAC addr are invalid as of now */
  xf->pm.lkup_dmac[0] = 0xff;

  /* Outer L3 */
  xf->l3m.ip.saddr = sip;
  xf->l3m.ip.daddr = rip;
  xf->l3m.source = udp->source;
  xf->l3m.dest = udp->dest;
  xf->pm.l4_off = xf->pm.l3_off + sizeof(*iph);
  
  return 0;
}


static int __always_inline
xdp2tc_has_xmd(void *md, struct xfi *xf)
{
  void *data      = DP_TC_PTR(DP_PDATA(md));
  void *data_meta = DP_TC_PTR(DP_MDATA(md));
  struct ll_xmdi *meta = data_meta;

  /* Check XDP gave us some data_meta */
  if (meta + 1 <= data) {
    if (meta->pi.skip != 0) {
      xf->pm.tc = 0;
      LLBS_PPLN_PASS(xf);
      return 1;
    }

    if (meta->pi.iport) {
      xf->pm.oport = meta->pi.iport;
      LLBS_PPLN_REWIRE(xf);
    } else {
      xf->pm.oport = meta->pi.oport;
      LLBS_PPLN_RDR(xf);
    }
    xf->pm.tc = 0;
    meta->pi.skip = 1;
    return 1;
  }

  return 0;
}

static int __always_inline
dp_tail_call(void *ctx,  struct xfi *xf, void *fa, __u32 idx)
{
  int z = 0;

  if (xf->l4m.ct_sts != 0) {
    return DP_PASS;
  }

#ifdef HAVE_DP_FC
  /* fa state can be reused */ 
  bpf_map_update_elem(&fcas, &z, fa, BPF_ANY);
#endif

  /* xfi state can be reused */ 
  bpf_map_update_elem(&xfis, &z, xf, BPF_ANY);

  bpf_tail_call(ctx, &pgm_tbl, idx);

  return DP_PASS;
}

#endif
