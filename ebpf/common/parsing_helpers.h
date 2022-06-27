/* SPDX-License-Identifier: (GPL-2.0-or-later OR BSD-2-clause) */
#ifndef __PARSING_HELPERS_H
#define __PARSING_HELPERS_H

#include <stddef.h>
#include <stdint.h>
#include <linux/if_ether.h>
#include <linux/if_packet.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/icmp.h>
#include <linux/icmpv6.h>
#include <linux/udp.h>
#include <linux/tcp.h>

#define __force __attribute__((force))

#ifndef memcpy
#define memcpy(dest, src, n) __builtin_memcpy((dest), (src), (n))
#define memset(dest, c, n) __builtin_memset((dest), (c), (n))
#endif

#define DP_ADD_PTR(x, len) ((void *)(((uint8_t *)((long)x)) + (len)))
#define DP_TC_PTR(x) ((void *)((long)x))
#define DP_DIFF_PTR(x, y) (((uint8_t *)DP_TC_PTR(x)) - ((uint8_t *)DP_TC_PTR(y)))

/* Header cursor to keep track of current parsing position */
struct hdr_cursor {
	void *pos;
};

#define VLAN_VID_MASK  0x0fff
#define VLAN_PCP_MASK  0xe000
#define VLAN_PCP_SHIFT 13

/*
 *	struct vlan_hdr - vlan header
 *	@h_vlan_TCI: priority and VLAN ID
 *	@h_vlan_encapsulated_proto: packet type ID or len
 */
struct vlan_hdr {
	__be16	h_vlan_TCI;
	__be16	h_vlan_encapsulated_proto;
};

#define ARP_ETH_HEADER_LEN 28

struct arp_ethheader {
    uint16_t  ar_hrd;         /* Hw type */
    uint16_t  ar_pro;         /* Proto type */
    uint8_t   ar_hln;         /* Hw addr len */
    uint8_t   ar_pln;         /* Proto addr len */
    uint16_t  ar_op;          /* Op-code. */

    uint8_t   ar_sha[6];      /* Sender hw addr */
    uint32_t  ar_spa;         /* Sender proto addr */
    uint8_t   ar_tha[6];      /* Target hw addr */
    uint32_t  ar_tpa;         /* Target proto addr */
} __attribute__((packed));

/* Shim L2 header for internal communication */
#define ETH_TYPE_LLB 0x9999

struct llb_ethheader {
    uint16_t iport;
    uint16_t oport;
    uint8_t  miss_table;
    uint8_t  rcode;
    uint16_t next_eth_type;
} __attribute__((packed));

#define MPLS_HEADER_LEN (4)
#define MPLS_LABEL_MASK ((1<<20)-1)
#define MPLS_TC_MASK    ((1<<4)-1)
#define MPLS_BOS_MASK   (1)
#define MPLS_TTL_MASK   (255)
#define MPLS_HDR_GET_LABEL(m) (bpf_ntohl((m)) & MPLS_LABEL_MASK)
#define MPLS_HDR_GET_TC(m)    ((bpf_ntohl((m))>>20) & MPLS_TC_MASK)
#define MPLS_HDR_GET_BOS(m)   ((bpf_ntohl((m))>>23) & MPLS_BOS_MASK)
#define MPLS_HDR_GET_TTL(m)   ((bpf_ntohl((m))>>24) & MPLS_TTL_MASK)

struct mpls_header {
    uint32_t mpls_tag;
};

#define VXLAN_UDP_DPORT (4789)
#define VXLAN_UDP_SPORT (4788)

/* VXLAN protocol header */
struct vxlan_hdr {
#define VXLAN_VI_FLAG_ON (bpf_htonl(0x08 << 24))
    __u32 vx_flags;
    __u32 vx_vni;
}__attribute__((packed));

/* Allow users of header file to redefine VLAN max depth */
#ifndef MAX_STACKED_VLANS
#define MAX_STACKED_VLANS 3
#endif

/*
 * Struct icmphdr_common represents the common part of the icmphdr and icmp6hdr
 * structures.
 */
struct icmphdr_common {
	__u8		type;
	__u8		code;
	__sum16	cksum;
};

/* IP flags. */
#define IP_CE		0x8000		/* Flag: "Congestion"		*/
#define IP_DF		0x4000		/* Flag: "Don't Fragment"	*/
#define IP_MF		0x2000		/* Flag: "More Fragments"	*/
#define IP_OFFSET	0x1FFF		/* "Fragment Offset" part	*/

static __always_inline int ip_is_fragment(const struct iphdr *iph)
{
	return (iph->frag_off & bpf_htons(IP_MF | IP_OFFSET)) != 0;
}

static __always_inline int ip_is_first_fragment(const struct iphdr *iph)
{
	return (iph->frag_off & bpf_htons(IP_OFFSET)) == 0;
}

static __always_inline int proto_is_vlan(__u16 h_proto)
{
	return !!(h_proto == bpf_htons(ETH_P_8021Q) ||
		  h_proto == bpf_htons(ETH_P_8021AD));
}

/* from include/net/ip.h */
static __always_inline int ip_decrease_ttl(struct iphdr *iph)
{
  __u32 check = iph->check;
  check += bpf_htons(0x0100);
  iph->check = (__u16)(check + (check >= 0xFFFF));
  return --iph->ttl;
}

static __always_inline __u16
csum_fold_helper(__u32 csum)
{
  return ~((csum & 0xffff) + (csum >> 16));
}

static __always_inline void
ipv4_csum(void *data_start,
          int data_size,
          __u32 *csum)
{
  *csum = bpf_csum_diff(0, 0, data_start, data_size, *csum);
  *csum = csum_fold_helper(*csum);
}

static __always_inline void
ipv4_l4_csum(void *data_start, __u32 data_size,
             __u64 *csum, struct iphdr *iph) {
  __u32 tmp = 0;
  *csum = bpf_csum_diff(0, 0, &iph->saddr, sizeof(__be32), *csum);
  *csum = bpf_csum_diff(0, 0, &iph->daddr, sizeof(__be32), *csum);
  // __builtin_bswap32 equals to htonl()
  tmp = __builtin_bswap32((__u32)(iph->protocol));
  *csum = bpf_csum_diff(0, 0, &tmp, sizeof(__u32), *csum);
  tmp = __builtin_bswap32((__u32)(data_size));
  *csum = bpf_csum_diff(0, 0, &tmp, sizeof(__u32), *csum);
  *csum = bpf_csum_diff(0, 0, data_start, data_size, *csum);
  *csum = csum_fold_helper(*csum);
}

#endif /* __PARSING_HELPERS_H */
