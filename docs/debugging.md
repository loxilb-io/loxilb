# loxilb - How to debug

* <b>Check loxilb logs</b>

loxilb logs its various important events and logs in the file /var/log/loxilb.log. Users can check it by using tail -f or any other command of choice. 

```
root@752531364e2c:/# tail -f /var/log/loxilb.log 
DBG:  2022/07/10 12:49:27 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:49:37 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:49:47 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:49:57 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:50:07 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:50:17 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:50:27 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:50:37 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:50:47 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
DBG:  2022/07/10 12:50:57 1:dst-10.10.10.1/32,proto-6,dport-2020,,do-dnat:eip-31.31.31.1,ep-5001,w-1,alive|eip-32.32.32.1,ep-5001,w-2,alive|eip-100.100.100.1,ep-5001,w-2,alive| pc 0 bc 0 
```


* <b>Check *loxicmd* to debug loxilb's internal state</b>

```
## Spawn a bash shell of loxilb docker 
docker exec -it loxilb bash

root@752531364e2c:/# loxicmd get lb       
| EXTERNALIP | PORT | PROTOCOL | SELECT | # OF ENDPOINTS |
|------------|------|----------|--------|----------------|
| 10.10.10.1 | 2020 | tcp      |      0 |              3 |


root@752531364e2c:/# loxicmd get lb -o wide
| EXTERNALIP | PORT | PROTOCOL | SELECT |  ENDPOINTIP   | TARGETPORT | WEIGHT |
|------------|------|----------|--------|---------------|------------|--------|
| 10.10.10.1 | 2020 | tcp      |      0 | 31.31.31.1    |       5001 |      1 |
|            |      |          |        | 32.32.32.1    |       5001 |      2 |
|            |      |          |        | 100.100.100.1 |       5001 |      2 |


root@0c4f9175c983:/# loxicmd get conntrack
| DESTINATIONIP |  SOURCEIP  | DESTINATIONPORT | SOURCEPORT | PROTOCOL |    STATE    | ACT |
|---------------|------------|-----------------|------------|----------|-------------|-----|
| 127.0.0.1     | 127.0.0.1  |           11111 |      47180 | tcp      | closed-wait |     |
| 127.0.0.1     | 127.0.0.1  |           11111 |      47182 | tcp      | est         |     |
| 32.32.32.1    | 31.31.31.1 |           35068 |      35068 | icmp     | bidir       |     |


root@65ad9b2f1b7f:/# loxicmd get port
| INDEX | PORTNAME |        MAC        | LINK/STATE  |    L3INFO     |    L2INFO     |
|-------|----------|-------------------|-------------|---------------|---------------|
|     1 | lo       | 00:00:00:00:00:00 | true/false  | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3801    |
|       |          |                   |             | IPv6 : []     |               |
|     2 | vlan3801 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3801    |
|       |          |                   |             | IPv6 : []     |               |
|     3 | llb0     | 42:6e:9b:7f:ff:36 | true/false  | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3803    |
|       |          |                   |             | IPv6 : []     |               |
|     4 | vlan3803 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3803    |
|       |          |                   |             | IPv6 : []     |               |
|     5 | eth0     | 02:42:ac:1e:01:c1 | true/true   | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3805    |
|       |          |                   |             | IPv6 : []     |               |
|     6 | vlan3805 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3805    |
|       |          |                   |             | IPv6 : []     |               |
|     7 | enp1     | fe:84:23:ac:41:31 | false/false | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3807    |
|       |          |                   |             | IPv6 : []     |               |
|     8 | vlan3807 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3807    |
|       |          |                   |             | IPv6 : []     |               |
|     9 | enp2     | d6:3c:7f:9e:58:5c | false/false | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3809    |
|       |          |                   |             | IPv6 : []     |               |
|    10 | vlan3809 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3809    |
|       |          |                   |             | IPv6 : []     |               |
|    11 | enp2v15  | 8a:9e:99:aa:f9:c3 | false/false | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3811    |
|       |          |                   |             | IPv6 : []     |               |
|    12 | vlan3811 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3811    |
|       |          |                   |             | IPv6 : []     |               |
|    13 | enp3     | f2:c7:4b:ac:fd:3e | false/false | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3813    |
|       |          |                   |             | IPv6 : []     |               |
|    14 | vlan3813 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3813    |
|       |          |                   |             | IPv6 : []     |               |
|    15 | enp4     | 12:d2:c3:79:f3:6a | false/false | Routed: false | IsPVID: true  |
|       |          |                   |             | IPv4 : []     | VID : 3815    |
|       |          |                   |             | IPv6 : []     |               |
|    16 | vlan3815 | aa:bb:cc:dd:ee:ff | true/true   | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 3815    |
|       |          |                   |             | IPv6 : []     |               |
|    17 | vlan100  | 56:2e:76:b2:71:48 | false/false | Routed: false | IsPVID: false |
|       |          |                   |             | IPv4 : []     | VID : 100     |
|       |          |                   |             | IPv6 : []     |               |

```
* <b>Debug loxilb kernel and eBPF components</b>

loxilb uses various eBPF maps as part of its DP implementation. These maps are pinned to OS filesystem and can be further used with bpftool to debug.

```
root@0c4f9175c983:/# ls -lart /opt/loxilb/dp/bpf/
total 0
-rw------- 1 root root 0 Jul 10 11:32 xfis
-rw------- 1 root root 0 Jul 10 11:32 xfck
-rw------- 1 root root 0 Jul 10 11:32 xctk
-rw------- 1 root root 0 Jul 10 11:32 tx_intf_stats_map
-rw------- 1 root root 0 Jul 10 11:32 tx_intf_map
-rw------- 1 root root 0 Jul 10 11:32 tx_bd_stats_map
-rw------- 1 root root 0 Jul 10 11:32 tmac_stats_map
-rw------- 1 root root 0 Jul 10 11:32 tmac_map
-rw------- 1 root root 0 Jul 10 11:32 smac_map
-rw------- 1 root root 0 Jul 10 11:32 sess_v4_stats_map
-rw------- 1 root root 0 Jul 10 11:32 sess_v4_map
-rw------- 1 root root 0 Jul 10 11:32 rt_v6_stats_map
-rw------- 1 root root 0 Jul 10 11:32 rt_v4_stats_map
-rw------- 1 root root 0 Jul 10 11:32 rt_v4_map
-rw------- 1 root root 0 Jul 10 11:32 polx_map
-rw------- 1 root root 0 Jul 10 11:32 pkts
-rw------- 1 root root 0 Jul 10 11:32 pkt_ring
-rw------- 1 root root 0 Jul 10 11:32 pgm_tbl
-rw------- 1 root root 0 Jul 10 11:32 nat_v4_map
-rw------- 1 root root 0 Jul 10 11:32 mirr_map
-rw------- 1 root root 0 Jul 10 11:32 intf_stats_map
-rw------- 1 root root 0 Jul 10 11:32 intf_map
-rw------- 1 root root 0 Jul 10 11:32 fcas
-rw------- 1 root root 0 Jul 10 11:32 fc_v4_stats_map
-rw------- 1 root root 0 Jul 10 11:32 fc_v4_map
-rw------- 1 root root 0 Jul 10 11:32 dmac_map
-rw------- 1 root root 0 Jul 10 11:32 ct_v4_map
-rw------- 1 root root 0 Jul 10 11:32 bd_stats_map
-rw------- 1 root root 0 Jul 10 11:32 acl_v6_stats_map
-rw------- 1 root root 0 Jul 10 11:32 acl_v4_stats_map
-rw------- 1 root root 0 Jul 10 11:32 acl_v4_map
drwxrwxrwt 3 root root 0 Jul 10 11:32 ..
lrwxrwxrwx 1 root root 0 Jul 10 11:32 xdp -> /opt/loxilb/dp/bpf//tc/
drwx------ 3 root root 0 Jul 10 11:32 tc
lrwxrwxrwx 1 root root 0 Jul 10 11:32 ip -> /opt/loxilb/dp/bpf//tc/


root@752531364e2c:/# bpftool map dump pinned /opt/loxilb/dp/bpf/intf_map 
[{
        "key": {
            "ifindex": 2,
            "ing_vid": 0,
            "pad": 0
        },
        "value": {
            "ca": {
                "act_type": 11,
                "ftrap": 0,
                "oif": 0,
                "cidx": 0
            },
            "": {
                "set_ifi": {
                    "xdp_ifidx": 1,
                    "zone": 0,
                    "bd": 3801,
                    "mirr": 0,
                    "polid": 0,
                    "r": [0,0,0,0,0,0
                    ]
                }
            }
        }
    },{
        "key": {
            "ifindex": 3,
            "ing_vid": 0,
            "pad": 0
        },
        "value": {
            "ca": {
                "act_type": 11,
                "ftrap": 0,
                "oif": 0,
                "cidx": 0
            },
            "": {
                "set_ifi": {
                    "xdp_ifidx": 3,
                    "zone": 0,
                    "bd": 3803,
                    "mirr": 0,
                    "polid": 0,
                    "r": [0,0,0,0,0,0
                    ]
                }
            }
        }
    }
]


root@752531364e2c:/# bpftool map dump pinned /opt/loxilb/dp/bpf/nat_v4_map
[{
        "key": {
            "daddr": 17435146,
            "dport": 58375,
            "zone": 0,
            "l4proto": 6
        },
        "value": {
            "ca": {
                "act_type": 5,
                "ftrap": 0,
                "oif": 0,
                "cidx": 1
            },
            "lock": {
                "val": 0
            },
            "nxfrm": 3,
            "sel_hint": 0,
            "sel_type": 0,
            "nxfrms": [{
                    "nat_flags": 0,
                    "inactive": 0,
                    "wprio": 1,
                    "res": 0,
                    "nat_xport": 35091,
                    "nat_xip": 18816799
                },{
                    "nat_flags": 0,
                    "inactive": 0,
                    "wprio": 2,
                    "res": 0,
                    "nat_xport": 35091,
                    "nat_xip": 18882592
                },{
                    "nat_flags": 0,
                    "inactive": 0,
                    "wprio": 2,
                    "res": 0,
                    "nat_xport": 35091,
                    "nat_xip": 23356516
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                },{
                    "nat_flags": 0,
                    "inactive": 1,
                    "wprio": 0,
                    "res": 0,
                    "nat_xport": 0,
                    "nat_xip": 0
                }
            ]
        }
    }
]
```
* <b>Check eBPF kernel debug logs</b>

Last but not the least, linux kernel outputs generic eBPF debug logs to /sys/kernel/debug/tracing/trace_pipe. Although loxilb eBPF modules do not emit logs in normal mode of operation, logs can be enabled after a recompilation. 

```
root@752531364e2c:/# cat /sys/kernel/debug/tracing/trace_pipe        
         loxicmd-30524   [001] d.s1 27870.170790: bpf_trace_printk: out-dir
         loxicmd-30524   [001] d.s1 27870.170791: bpf_trace_printk: smr 4
         loxicmd-30529   [000] d.s1 27871.617467: bpf_trace_printk: [CTRK] start

         loxicmd-30529   [000] d.s1 27871.617484: bpf_trace_printk: new-ct4
         loxicmd-30529   [000] d.s1 27871.617486: bpf_trace_printk: in-dir
         loxicmd-30529   [000] d.s1 27871.617488: bpf_trace_printk: smr 0
         loxicmd-30529   [000] d.s1 27871.617503: bpf_trace_printk: [CTRK] start

         loxicmd-30529   [000] d.s1 27871.617503: bpf_trace_printk: out-dir
         loxicmd-30529   [000] d.s1 27871.617504: bpf_trace_printk: smr 4
            sshd-30790   [000] d.s1 27970.031847: bpf_trace_printk: [CTRK] start

            sshd-30790   [000] d.s1 27970.031866: bpf_trace_printk: new-ct4
            sshd-30790   [000] d.s1 27970.031868: bpf_trace_printk: in-dir
            sshd-30790   [000] d.s1 27970.031870: bpf_trace_printk: smr 0
            sshd-30790   [000] d.s1 27970.031887: bpf_trace_printk: [CTRK] start

            sshd-30790   [000] d.s1 27970.031887: bpf_trace_printk: out-dir
            sshd-30790   [000] d.s1 27970.031888: bpf_trace_printk: smr 0
            sshd-30790   [000] d.s1 27970.031900: bpf_trace_printk: [CTRK] start

```






