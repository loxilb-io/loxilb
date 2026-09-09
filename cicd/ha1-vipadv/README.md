# ha1-vipadv

Regression case for VIP advertisement: which interface a load balancer rule's
VIP is bound to, advertised on, and removed from as HA state changes.

Derived from `ha1`, which supplies the topology this needs — a VIP that is not
in any subnet the load balancers hold, reachable only through a static route on
the upstream router.

## Topology

```
                        vlan11 (bridge, 11.11.11.0/24, 2001:db8:11::/64)
  user ---- r1 ---------+---- llb1
  1.1.1.1   1.1.1.254   |     11.11.11.1   2001:db8:11::1
                        +---- llb2
                        |     11.11.11.2   2001:db8:11::2
                        +---- ep1  11.11.11.3
                        +---- ep2  11.11.11.4
                        +---- ep3  11.11.11.5
```

Two VIPs, each covering a different resolution path:

| VIP | Reachability | Bound on |
| --- | --- | --- |
| `20.20.20.1` | routed, `r1` holds a static `/32` towards the VLAN | `lo` |
| `2001:db8:11::100` | L2 adjacent, inside the VLAN 11 subnet | `vlan11` |

`20.20.20.1` is in no subnet either load balancer holds, and neither has a route
to it. That is the case where an advertise interface cannot be derived from the
local addresses alone.

## What it checks

Before and after an HA transition:

- the IPv4 VIP is bound on `lo` on the master and absent on the backup
- the IPv6 VIP sits on `vlan11` on the master and is absent on the backup
- which interface each VIP resolved to, read back from the load balancer's log
- a gratuitous ARP for the IPv4 VIP reaches `r1` on the VLAN, and reaches it
  again on a later sweep

The repeat matters. A resolver that asks the kernel for a route to the VIP gets
`local ... dev lo` back once the VIP is bound, so it works on the first pass and
goes wrong on every one after it. The VIP sweep runs roughly every 40 seconds,
so the second capture window is what a first pass alone would not catch.

## Modes

`VIP_ADV_DEV` selects how the advertise interface is chosen. It is passed to
both scripts.

```bash
./config.sh && ./validation.sh && ./rmconfig.sh              # --vip-adv-dev vlan11
VIP_ADV_DEV= ./config.sh && VIP_ADV_DEV= ./validation.sh     # automatic
```

Default is `vlan11`, which pins the advertisement with `--vip-adv-dev`.

With `VIP_ADV_DEV` empty the load balancers resolve on their own, and in this
topology they land on the container's default route, not the VLAN. That is the
correct answer for the routing table they have: neither has a route to
`20.20.20.1`, so the only main table route covering it is the default one. It is
also why the option exists. The gratuitous ARP assertion is dropped in this
mode; the rest still applies, and the IPv4 VIP is still expected on `lo` — an
interface that cannot be advertised on must not stop the VIP from being bound.

## HA transitions

State is driven through the load balancer's own API rather than keepalived:

```
POST /netlox/v1/config/cistate {"instance":"llb-inst0","state":"BACKUP", ...}
```

`llb-inst0` is the instance a rule created without one belongs to. The API
returns instances in no fixed order, so the scripts select it by name.

The load balancers are still spawned in cluster mode, with each other as peers,
which is what settles the initial master and backup. What the case does not use
is the separate keepalived container `ha1` expects: its failover is a
`docker restart ka_$master`, and no such container is started here.

That has a second consequence. The case does not exercise the data path, because
`ha1`'s traffic test uses the keepalived VRRP address as the endpoint gateway
and that address does not exist without it.

## Out of scope

- data path correctness, per above
- resolution of a *routed* IPv6 VIP. The IPv6 VIP here is L2 adjacent, so it
  exercises the address-follows-the-port rule and the removal path, not a route
  lookup. A routed IPv6 VIP needs IPv6 routing on the upstream router.

## Requirements

`tcpdump` on the host. The gratuitous ARP capture runs in `r1`'s namespace with
the host's binary; the check is skipped, not failed, when it is missing.
