# tlsproxyprotov2 — TLS 1.3 + PROXY protocol v2 reproduction

Reproduces loxilb Issue **#1044** / Discussion **#1089** and Issue **#675**: with a load
balancer in `fullnat` mode and PROXY protocol v2 enabled (`--ppv2en`), the **TLS 1.3
handshake fails** while TLS 1.2 works.

Two *independent* defects in the same eBPF helper (`dp_ins_ppv2()`) are covered:

| case | trigger | issue |
|------|---------|-------|
| `REPRO`   | first data packet is a GSO super-packet | #1044 / #1089 |
| `REPRO-2` | client sends no TCP options (`doff == 20`) | #675 (Windows-only) |

## REPRO-2 — the Windows failure (#675)

`dp_ins_ppv2()` opens a 28-byte hole with `bpf_skb_adjust_room()` and then picks where to
write the PROXY header based on the TCP **data offset**, because the header goes after the
TCP options. That dispatch had arms for `doff` 24/28/32/36/40 and a drop for anything
larger — but **no arm for `doff == 20`** (no options at all), so the hole was left holding
`skb_push()` leftovers (stale IP/TCP header bytes) and those bytes never entered the
checksum either.

Which clients hit it is decided by the **TCP timestamp option**:

| client | timestamps | `doff` | result |
|--------|-----------|--------|--------|
| Linux / macOS / Android, and curl under WSL | on by default | 32 | works |
| **Windows 10 / 11** | **off by default** | **20** | **fails** |

That is why the same Windows box worked through WSL and why disabling TSO/GSO/GRO never
helped — this defect has nothing to do with offload. Depending on whether the path
validates TCP checksums, the backend either drops the segment (connection stalls, backend
keeps `ACK 1` and only SACKs the later data) or accepts it and nginx logs
`broken header while reading PROXY protocol`. Both symptoms appear in issue #675.

`REPRO-2` runs at **default MTU with offload ON** — i.e. it is `CONTROL-1` with exactly one
variable changed (`sysctl net.ipv4.tcp_timestamps=0` in the client netns), which makes it
decisive.

Root-cause analysis: [`../../docs/tls13-proxyproto-v2-issue/`](../../docs/tls13-proxyproto-v2-issue/).
The bug is in the eBPF L4 datapath `dp_ins_ppv2()`, which inserts the 28-byte PROXY v2
header into the first TCP data packet (the TLS ClientHello) with
`bpf_skb_adjust_room(..., BPF_F_ADJ_ROOM_FIXED_GSO)`. **When that packet is a GSO
super-packet (`gso_size>0`) the insertion corrupts the byte stream**, so the backend
can't parse the PROXY header / TLS record and the handshake dies right after ClientHello.

## Reproduced (2026-07-15) — pure docker/veth

Earlier this scenario was a *passing baseline* because at the default MTU the ClientHello
fits in one segment (`< MSS`) → it is never a GSO skb → the `FIXED_GSO` path is never
taken. `validation.sh` now **forces the GSO path** and reproduces the bug:

1. Shrink the client's send-MSS to the VIP (small per-route PMTU, `REPRO_MTU=320` → MSS ~256).
2. Inflate the TLS 1.3 ClientHello past that MSS (long `-servername`, ~500 B).

so the client's GSO builds a `gso_size>0` super-packet for the ClientHello.

### The control makes it decisive (it is GSO, not MTU/PMTU)

| case | condition | result |
|------|-----------|--------|
| CONTROL-1 | default MTU, offload ON — hello is single-segment | **OK** |
| CONTROL-2 | `MTU=320`, offload **OFF** — real multi-segment, not GSO | **OK** |
| REPRO     | `MTU=320`, offload **ON** — GSO super-packet ClientHello | **FAIL** |
| REPRO-2   | default MTU, offload ON, client TCP timestamps **off** (`doff=20`) | **FAIL** |

Same MTU, same hello: only client segmentation-offload differs. Offload OFF (real
separate segments) succeeds; offload ON (one GSO super-packet) fails. That rules out
"small MTU / PMTU" and pins the failure on GSO super-packet handling in `dp_ins_ppv2`.

### Client-observed symptom

With the nginx backend (`listen ... ssl proxy_protocol`), the client sees the handshake
die immediately after ClientHello:
- `openssl s_client -tls1_3` → connection stalls, then closes (the **`SSL_ERROR_SYSCALL`**
  form reported in Discussion #1089).
- backend nginx logs: `broken header: "" while reading PROXY protocol` — proof the bytes
  reached the backend but the inserted PROXY header was corrupted.

> Note on the exact #1044 string: `error:0A00042E:...ssl3_read_bytes:tlsv1 alert protocol
> version` is emitted when the backend gets *past* the PROXY header but then reads a
> byte-shifted TLS record. Here nginx rejects earlier, at the PROXY-parse stage
> (`broken header`), so it closes the connection instead of sending that TLS alert.
> Both are the **same root cause** (GSO + `dp_ins_ppv2`); which one you see depends on
> where the GSO corruption lands and how the backend (nginx vs Traefik) reacts.

## Run

```
cd cicd/tlsproxyprotov2/
./config.sh          # topology + nginx(proxy_protocol,TLS1.2/1.3) + fullnat/ppv2 rule
./validation.sh      # regression gate: all four cases must pass (EXPECT=fixed, default)
./rmconfig.sh
```

Knobs:
- `REPRO_MTU=<n> ./validation.sh` — change the forced client PMTU.
- `EXPECT=bug ./validation.sh` — invert the gate: pass when a defect still reproduces.
  Use it against a pre-fix image, e.g.
  `LOXILB_IMAGE=ghcr.io/loxilb-io/loxilb:v0.9.8.8 ./config.sh && EXPECT=bug ./validation.sh`
- `LOXILB_IMAGE=<img> ./config.sh` — run the testbed on a specific image.
- `BASE_IMAGE=<img> ./build_deploy.sh` — compile `loxilb-ebpf` from the working tree, layer
  the objects onto `<img>` as `loxilb:ppv2test`, and redeploy the testbed.

## Topology / what it exercises (vs cicd/httpsproxy)

- `llb1` runs in **normal eBPF mode** (no `--proxyonlymode`) → the fast-path `dp_ins_ppv2` runs.
- LB rule is `--mode=fullnat --ppv2en` (no `--security`) → loxilb does **not** terminate TLS.
- Backends run **nginx** with `listen 8080 ssl proxy_protocol;` (TLS 1.2 + 1.3): nginx strips
  the PROXY header loxilb inserts, then terminates TLS. Mirrors the real-world reports
  (nginx / Traefik). This is deliberately different from `cicd/httpsproxy`
  (`--mode=fullproxy --security=https`, the L7 OpenSSL path, which would mask the bug).

> Debugging trap: `llb1` is `--privileged`, so `docker exec llb1 ip link set <veth> mtu <N>`
> succeeds. Accidentally lowering an llb1 veth MTU blackholes the backend's larger response
> and looks like a datapath bug — it isn't. Restore MTU 9000.

> Note on checksums in this testbed: the docker/veth path does not validate TCP checksums
> (every packet shows `incorrect` under `tcpdump -vv`, the usual `CHECKSUM_PARTIAL`
> artifact), so here a corrupt insertion shows up as *content* corruption — nginx's
> `broken header` — rather than as the silent drop / SACK stall seen on physical NICs.
> Same defect, different observable.

## CI status

GREEN once both fixes are in the datapath; `./validation.sh` (default `EXPECT=fixed`) is
the regression gate. Not yet wired into a GitHub workflow; run manually, or promote it to a
sanity workflow.

Verified 2026-08-14 on `ghcr.io/loxilb-io/loxilb:v0.9.8.8`:

| case | v0.9.8.8 | + `doff == 20` fix |
|------|----------|--------------------|
| CONTROL-1 | OK | OK |
| CONTROL-2 | OK | OK |
| REPRO (GSO)     | OK (fixed by loxilb-ebpf#87) | OK |
| REPRO-2 (doff=20) | **FAIL** | OK |

The 28-byte segment loxilb emits toward the backend, `doff == 20`, before vs after:

```
before: 0000 0000 0000 0000 0000 93e2 1f90 d3a9 b421 9cea deb0 5010 01ea 5543
        ^ skb_push leftovers: zero padding + a copy of the TCP header
after:  0d0a 0d0a 000d 0a51 5549 540a 2111 000c 0a0a 0a01 0a0a 0afe ba60 07e4
        ^ PROXY v2 signature, TCP/IPv4, len=12, 10.10.10.1 -> 10.10.10.254:2020
```
