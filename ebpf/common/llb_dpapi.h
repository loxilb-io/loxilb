/*
 *  llb_dpapi.h: LoxiLB DP Application Programming Interface 
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 *  SPDX-License-Identifier: (GPL-2.0-or-later OR BSD-2-clause) 
 */
#ifndef __LLB_DPAPI_H__
#define __LLB_DPAPI_H__

#define LLB_MGMT_CHANNEL      "llb0"
#define LLB_SECTION_PASS      "xdp_pass"
#define LLB_FP_IMG_DEFAULT    "/opt/loxilb/llb_xdp_main.o"
#define LLB_FP_IMG_BPF        "/opt/loxilb/llb_ebpf_main.o"
#define LLB_DB_MAP_PDIR       "/opt/loxilb/dp/bpf"

#define LLB_MIRR_MAP_ENTRIES  (32)
#define LLB_NH_MAP_ENTRIES    (4*1024)
#define LLB_RTV4_MAP_ENTRIES  (32*1024)
#define LLB_RTV4_PREF_LEN     (48)
#define LLB_ACLV4_MAP_ENTRIES (256*1024)
#define LLB_ACLV6_MAP_ENTRIES (2*1024)
#define LLB_RTV6_MAP_ENTRIES  (2*1024)
#define LLB_TMAC_MAP_ENTRIES  (2*1024)
#define LLB_DMAC_MAP_ENTRIES  (8*1024)
#define LLB_NATV4_MAP_ENTRIES (4*1024)
#define LLB_NATV4_STAT_MAP_ENTRIES (4*16*1024) /* 16 end-points */
#define LLB_SMAC_MAP_ENTRIES  (LLB_DMAC_MAP_ENTRIES)
#define LLB_INTERFACES        (512)
#define LLB_PORT_NO           (LLB_INTERFACES-1)
#define LLB_PORT_PIDX_START   (LLB_PORT_NO - 128)
#define LLB_INTF_MAP_ENTRIES  (6*1024)
#define LLB_FCV4_MAP_ENTRIES  (256*1024)
#define LLB_CTV4_MAP_ENTRIES  (LLB_FCV4_MAP_ENTRIES)
#define LLB_PGM_MAP_ENTRIES   (8)
#define LLB_FCV4_MAP_ACTS     (DP_SET_TOCP)
#define LLB_POL_MAP_ENTRIES   (8*1024)
#define LLB_SESS_MAP_ENTRIES  (20*1024)
#define LLB_PSECS             (8)
#define LLB_MAX_NXFRMS        (16)

#define LLB_DP_CT_PGM_ID       (2)
#define LLB_DP_PKT_SLOW_PGM_ID (1)
#define LLB_DP_PKT_PGM_ID      (0)

/* Hard-timeout of 40s for fc dp entry */
#define FC_V4_DPTO            (40000000000)

/* Hard-timeout of 2m for fc cp entry */
#define FC_V4_CPTO            (120000000000)

/* Hard-timeout of 30m for ct entry */
#define CT_V4_CPTO            (1800000000000)

/* Hard-timeouts for ct xxx entry */
#define CT_TCP_FN_CPTO        (60000000000)
#define CT_SCTP_FN_CPTO       (60000000000)
#define CT_UDP_FN_CPTO        (60000000000)
#define CT_ICMP_FN_CPTO       (40000000000)

enum llb_dp_tid {
  LL_DP_INTF_MAP = 0,
  LL_DP_INTF_STATS_MAP,
  LL_DP_BD_STATS_MAP,
  LL_DP_SMAC_MAP,
  LL_DP_TMAC_MAP,
  LL_DP_ACLV4_MAP,
  LL_DP_RTV4_MAP,
  LL_DP_NH_MAP,
  LL_DP_DMAC_MAP,
  LL_DP_TX_INTF_MAP,
  LL_DP_MIRROR_MAP,
  LL_DP_TX_INTF_STATS_MAP,
  LL_DP_TX_BD_STATS_MAP,
  LL_DP_PKT_PERF_RING,
  LL_DP_RTV4_STATS_MAP,
  LL_DP_RTV6_STATS_MAP,
  LL_DP_ACLV4_STATS_MAP,
  LL_DP_ACLV6_STATS_MAP,
  LL_DP_TMAC_STATS_MAP,
  LL_DP_FCV4_MAP,
  LL_DP_FCV4_STATS_MAP,
  LL_DP_PGM_MAP,
  LL_DP_POL_MAP,
  LL_DP_CTV4_MAP,
  LL_DP_NAT4_MAP,
  LL_DP_NAT4_STATS_MAP,
  LL_DP_SESS4_MAP,
  LL_DP_SESS4_STATS_MAP,
  LL_DP_MAX_MAP
};

enum {
  DP_SET_DROP            = 0,
  DP_SET_RM_VXLAN        = 1,
  DP_SET_RT_TUN_NH       = 2,
  DP_SET_L3RT_TUN_NH     = 3,
  DP_SET_SNAT            = 4,
  DP_SET_DNAT            = 5,
  DP_SET_NEIGH_L2        = 6,
  DP_SET_NEIGH_VXLAN     = 7,
  DP_SET_ADD_L2VLAN      = 8,
  DP_SET_RM_L2VLAN       = 9,
  DP_SET_TOCP            = 10,
  DP_SET_IFI             = 11,
  DP_SET_NOP             = 12,
  DP_SET_LOCAL_STACK     = 13,
  DP_SET_L3_EN           = 14,
  DP_SET_RT_NHNUM        = 15,
  DP_SET_SESS_FWD_ACT    = 16,
  DP_SET_RDR_PORT        = 17,
  DP_SET_POLICER         = 18,
  DP_SET_DO_POLICER      = 19,
  DP_SET_FCACT           = 20,
  DP_SET_DO_CT           = 21,
  DP_SET_RM_GTP          = 22,
  DP_SET_ADD_GTP         = 23
};

struct dp_cmn_act {
  __u8 act_type;
  __u8 ftrap;
  __u16 oif;
  __u32 cidx;
};

struct dp_rt_l2nh_act {
  __u8 dmac[6];
  __u8 smac[6];
  __u16 bd;  
  __u16 rnh_num;
};

struct dp_rt_nh_act {
  __u16 nh_num;
  __u16 bd; 
  __u32 tid;
  struct dp_rt_l2nh_act l2nh;
};

struct dp_rt_l3tun_act {
  __u32 rip;
  __u32 sip;
  __u32 tid;
  __u32 aux;
};

struct dp_rt_l2vxnh_act {
  struct dp_rt_l3tun_act l3t;
  struct dp_rt_l2nh_act l2nh;
};

struct dp_rdr_act {
  __u16 oport;
  __u16 fr;
};

struct dp_l2vlan_act {
  __u16 vlan;
  __u16 oport;
};

struct dp_sess_act {
  __u32 sess_id;
};

struct dp_nat_act {
  __u32 xip;
  __u16 xport;
  __u8 fr;
  __u8 doct;
  __u32 rid;
  __u32 aid;
};

#define MIN_DP_POLICER_RATE  (8*1000*1000)  /* 1 MBps = 8 Mbps */

struct dp_pol_stats {
  uint64_t drop_packets;
  uint64_t pass_packets;
};

struct dp_policer_act {
  __u8  trtcm;
  __u8  color_aware;
  __u16 drop_prio; 
  __u32 pad;
  __u32 cbs;
  __u32 ebs;

  /* Internal state data */
  __u32 tok_c;
  __u32 tok_e;
  __u64 toksc_pus;
  __u64 tokse_pus;
  __u64 lastc_uts;
  __u64 laste_uts;
  struct dp_pol_stats ps;
};

struct dp_nh_key {
  __u32 nh_num;
};

struct dp_nh_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         * DP_SET_NEIGH_L2
                         */
  union {
    struct dp_rt_l2nh_act rt_l2nh;
    struct dp_rt_l2vxnh_act rt_l2vxnh;
  };
};

struct dp_rtv4_key {
  struct bpf_lpm_trie_key l;
  union {
    __u8  v4k[6];
    __u32 addr; 
  };
}__attribute__((packed));

struct dp_rt_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         *  DP_SET_DROP
                         *  DP_SET_TOCP
                         *  DP_SET_RDR_PORT
                         *  DP_SET_RT_NHNUM
                         *  DP_SET_RT_TUN_NH
                         */
  union {
    struct dp_rdr_act port_act;
    struct dp_rt_nh_act rt_nh;
  };
};


struct dp_fcv4_key {
  __u8  smac[6];
  __u8  dmac[6];
  __u8  in_smac[6];
  __u8  in_dmac[6];

  __u32 daddr; 
  __u32 saddr; 
  __u16 sport; 
  __u16 dport; 
  __u32 in_port;

  __u8  l4proto;
  __u8  in_l4proto;
  __u16 in_sport; 
  __u32 in_daddr; 

  __u32 in_saddr; 
  __u16 in_dport; 
  __u16 bd;
};

struct dp_fc_tact {
  struct dp_cmn_act ca; /* Possible actions : See below */
  union {
    struct dp_rdr_act port_act;
    struct dp_rt_nh_act nh_act;          /* DP_SET_RM_VXLAN
                                          * DP_SET_RT_TUN_NH
                                          * DP_SET_L3RT_TUN_NH
                                          */
    struct dp_nat_act nat_act;           /* DP_SET_SNAT, DP_SET_DNAT */
    struct dp_rt_l2nh_act nl2;           /* DP_SET_NEIGH_L2 */
    struct dp_rt_l2vxnh_act nl2vx;       /* DP_SET_NEIGH_VXLAN */
    struct dp_l2vlan_act l2ov;           /* DP_SET_ADD_L2VLAN,
                                          * DP_SET_RM_L2VLAN
                                          */
  };
};

struct dp_fc_tacts {
  struct dp_cmn_act ca;
  __u64 its;
  struct dp_fc_tact fcta[LLB_FCV4_MAP_ACTS];
};

struct dp_dmac_key {
  __u8 dmac[6];
  __u16 bd;
};

struct dp_dmac_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         *  DP_SET_DROP
                         *  DP_SET_RDR_PORT
                         *  DP_SET_ADD_L2VLAN
                         *  DP_SET_RM_L2VLAN
                         */
  union {
    struct dp_l2vlan_act vlan_act;
    struct dp_rdr_act port_act;
  };
};

struct dp_tmac_key {
  __u8 mac[6];
  __u8 tun_type;
  __u8 pad;
  __u32 tunnel_id;
};

struct dp_tmac_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         * DP_SET_DROP 
                         * DP_SET_TMACT_HIT
                         */
  union {
    struct dp_rt_nh_act rt_nh;
  };
};

struct dp_smac_key {
  __u8 smac[6];
  __u16 bd;
};

struct dp_smac_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         * DP_SET_DROP 
                         * DP_SET_TOCP
                         * DP_SET_NOP
                         */
};

struct intf_key {
  __u32 ifindex;
  __u16 ing_vid;
  __u16 pad;
};

struct dp_intf_tact_set_ifi {
  __u16 xdp_ifidx;
  __u16 zone;
  __u16 bd;
  __u16 mirr;
  __u16 polid;
  __u8  pprop;
  __u8  r[5];
};

struct dp_intf_tact {
  struct dp_cmn_act ca;
  union {
    struct dp_intf_tact_set_ifi set_ifi;
  };
};

struct dp_intf_map {
	struct intf_key key;
  struct dp_intf_tact acts;
};

struct dp_mirr_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         * DP_SET_NEIGH_VXLAN
                         * DP_SET_ADD_L2VLAN
                         * DP_SET_RM_L2VLAN
                         */
  union {
    struct dp_rt_l2vxnh_act rt_l2vxnh;
    struct dp_l2vlan_act vlan_act;
    struct dp_rdr_act port_act;
  };
};

struct dp_pol_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         * DP_SET_DO_POLICER
                         */
  struct bpf_spin_lock lock;
  union {
    struct dp_policer_act pol;
  };
};

struct dp_pb_stats {
  uint64_t bytes;
  uint64_t packets;
};
typedef struct dp_pb_stats dp_pb_stats_t;

struct dp_pbc_stats {
  dp_pb_stats_t st;
  int used;
};
typedef struct dp_pbc_stats dp_pbc_stats_t;

/* Connection tracking related defines */
typedef enum {
  CT_DIR_IN = 0,
  CT_DIR_OUT,
  CT_DIR_MAX
} ct_dir_t;

typedef enum {
  CT_STATE_NONE = 0x0,
  CT_STATE_REQ  = 0x1,
  CT_STATE_REP  = 0x2,
  CT_STATE_EST  = 0x4,
  CT_STATE_FIN  = 0x8,
  CT_STATE_DOR  = 0x10
} ct_state_t;

typedef enum {
  CT_FSTATE_NONE = 0x0,
  CT_FSTATE_SEEN = 0x1,
  CT_FSTATE_DOR  = 0x2
} ct_fstate_t;

typedef enum {
  CT_SMR_ERR    = -1,
  CT_SMR_INPROG = 0,
  CT_SMR_EST    = 1,
  CT_SMR_UEST   = 2,
  CT_SMR_FIN    = 3,
  CT_SMR_CTD    = 4,
  CT_SMR_UNT    = 100,
  CT_SMR_INIT   = 200,
} ct_smr_t;

#define CT_TCP_FIN_MASK (CT_TCP_FINI|CT_TCP_FINI2|CT_TCP_FINI3|CT_TCP_CW)

typedef enum {
  CT_TCP_CLOSED = 0x0,
  CT_TCP_SS     = 0x1,
  CT_TCP_SA     = 0x2,
  CT_TCP_EST    = 0x4,
  CT_TCP_FINI   = 0x10,
  CT_TCP_FINI2  = 0x20,
  CT_TCP_FINI3  = 0x40,
  CT_TCP_CW     = 0x80,
  CT_TCP_ERR    = 0x100
} ct_tcp_state_t;

typedef struct {
  __u32 hstate;
  __u32 seq;
#define CT_TCP_INIT_ACK_THRESHOLD 3
  __u16 init_acks;
} ct_tcp_pinfd_t;

typedef struct {
  ct_tcp_state_t state;
  ct_dir_t fndir;
  ct_tcp_pinfd_t tcp_cts[CT_DIR_MAX];
} ct_tcp_pinf_t;

typedef enum {
  CT_UDP_CNI    = 0x0,
  CT_UDP_UEST   = 0x1,
  CT_UDP_EST    = 0x2,
  CT_UDP_FINI   = 0x8,
} ct_udp_state_t;

typedef struct {
  __u16 state;
#define CT_UDP_CONN_THRESHOLD 4
  __u16 pkts_seen;
  __u16 rpkts_seen;
} ct_udp_pinf_t;

typedef enum {
  CT_ICMP_CLOSED= 0x0,
  CT_ICMP_REQS  = 0x1,
  CT_ICMP_REPS  = 0x2,
  CT_ICMP_FINI  = 0x4,
  CT_ICMP_DUNR  = 0x8,
  CT_ICMP_TTL   = 0x10,
  CT_ICMP_RDR   = 0x20,
  CT_ICMP_UNK   = 0x40,
} ct_icmp_state_t;

typedef struct {
  __u32 hstate;
  __u32 seq;
  __u16 init_acks;
} ct_sctp_pinfd_t;

#define CT_SCTP_FIN_MASK (CT_SCTP_SHUT|CT_SCTP_SHUTA|CT_SCTP_SHUTC|CT_SCTP_ABRT)

typedef enum {
  CT_SCTP_CLOSED  = 0x0,
  CT_SCTP_INIT    = 0x1,
  CT_SCTP_INITA   = 0x2,
  CT_SCTP_COOKIE  = 0x4,
  CT_SCTP_COOKIEA = 0x10,
  CT_SCTP_EST     = 0x10,
  CT_SCTP_SHUT    = 0x20,
  CT_SCTP_SHUTA   = 0x40,
  CT_SCTP_SHUTC   = 0x80,
  CT_SCTP_ERR     = 0x100,
  CT_SCTP_ABRT    = 0x200
} ct_stcp_state_t;

typedef struct {
  ct_stcp_state_t state;
  ct_dir_t fndir;
  uint32_t itag;
  uint32_t otag;
  uint32_t cookie;
  ct_sctp_pinfd_t stcp_cts[CT_DIR_MAX];
} ct_sctp_pinf_t;

typedef struct {
  uint8_t state;
  uint8_t errs;
  uint16_t lseq;
} ct_icmp_pinf_t;

typedef struct {
  ct_state_t state;
} ct_l3inf_t;

typedef struct {
  union {
    ct_tcp_pinf_t t;
    ct_udp_pinf_t u;
    ct_icmp_pinf_t i;
    ct_sctp_pinf_t s;
  };
  __u32 frag;
  ct_l3inf_t l3i;
} ct_pinf_t;

struct mf_xfrm_inf
{
  /* LLB_NAT_XXX flags */
  uint8_t nat_flags;
  uint8_t inactive;
  uint16_t wprio;
  uint16_t res;
  uint16_t nat_xport;
  uint32_t nat_xip;
};
typedef struct mf_xfrm_inf nxfrm_inf_t;

struct dp_ctv4_dat {
  __u32 rid;
  __u32 aid;
  ct_pinf_t pi;
  ct_dir_t dir;
  ct_smr_t smr;
  nxfrm_inf_t xi;
  dp_pb_stats_t pb;
};

struct dp_aclv4_tact {
  struct dp_cmn_act ca; /* Possible actions :
                         *  DP_SET_DROP
                         *  DP_SET_TOCP
                         *  DP_SET_NOP
                         *  DP_SET_RDR_PORT
                         *  DP_SET_RT_NHNUM
                         *  DP_SET_SESS_FWD_ACT
                         */
  struct bpf_spin_lock lock;
  struct dp_ctv4_dat ctd;
  __u64 lts;            /* Last used timestamp */
  union {
    struct dp_rdr_act port_act;
    struct dp_sess_act pdr_sess_act;
    struct dp_rt_nh_act rt_nh;
    struct dp_nat_act nat_act;
  };
};

struct dp_aclv4_tact_set {
  uint16_t wp;
  uint16_t fc;
  uint32_t tc;
  struct dp_aclv4_tact tact;
};

#define ACL_V4_MAX_ACT_SET     16 

#define DP_SET_LB_NONE         0
#define DP_SET_LB_WPRIO        1
#define DP_SET_LB_RR           2

struct dp_aclv4_tacts {
  uint16_t num_acts;
  uint16_t lb_type;
  uint32_t rdata;
  struct dp_aclv4_tact_set act_set[ACL_V4_MAX_ACT_SET];
};
typedef struct dp_aclv4_tacts dp_aclv4_tacts_t;

struct dp_ctv4_key {
  __u32 daddr;
  __u32 saddr;
  __u16 sport;
  __u16 dport;
  __u16 zone;
  __u8  l4proto;
  __u8  r;
};

struct dp_natv4_key {
  __u32 daddr;
  __u16 dport;
  __u16 zone;
  __u8  l4proto;
};

#define NAT_LB_SEL_RR   0
#define NAT_LB_SEL_HASH 1
#define NAT_LB_SEL_PRIO 2

struct dp_natv4_tacts {
  struct dp_cmn_act ca;
  struct bpf_spin_lock lock;
  uint32_t nxfrm;
  uint16_t sel_hint;
  uint16_t sel_type;
  struct mf_xfrm_inf nxfrms[LLB_MAX_NXFRMS];
};

/* This is currently based on ULCL classification scheme */
struct dp_sess4_key {
  __u32 daddr;
  __u32 saddr;
  __u32 teid;
  __u32 r;
};

struct dp_sess_tact {
  struct dp_cmn_act ca;
  uint8_t qfi; 
  uint8_t r1;
  uint16_t r2;
  uint32_t rip;
  uint32_t sip;
  uint32_t teid;
};

struct ll_dp_pmdi {
  __u32 ifindex;
  __u16 xdp_inport;
  __u8  table_id;
  __u8  rcode;
  __u16 pkt_len;
  __u16 xdp_oport;
  __u8  pad[4];   /* Align to 64-bit boundary */
  uint8_t data[];
}; 

struct dp_map_ita {
  void *next_key;
  void *val;
  void *uarg;
};
typedef struct dp_map_ita dp_map_ita_t;

/* Policer map stats update callback */
typedef void (*dp_pts_cb_t)(uint32_t idx, struct dp_pol_stats *ps);
/* Map stats update callback */
typedef void (*dp_ts_cb_t)(uint32_t idx, uint64_t bc, uint64_t pc);
/* Map stats idx valid check callback */
typedef int (*dp_tiv_cb_t)(int tid, uint32_t idx);
/* Map walker */
typedef int (*dp_map_walker_t)(int tid, void *key, void *arg);

int llb_map2fd(int t);
int llb_fetch_map_stats_cached(int tbl, uint32_t index, int raw, void *bc, void *pc);
void llb_age_map_entries(int tbl);
void llb_collect_map_stats(int tbl);
int llb_fetch_pol_map_stats(int tid, uint32_t e, void *ppass, void *pdrop);
void llb_clear_map_stats(int tbl, __u32 idx);
int llb_add_map_elem(int tbl, void *k, void *v);
int llb_del_map_elem(int tbl, void *k);
void llb_map_loop_and_delete(int tbl, dp_map_walker_t cb, dp_map_ita_t *it);
int llb_dp_link_attach(const char *ifname, const char *psec, int mp_type, int unload);

#endif /* __LLB_DPAPI_H__ */
