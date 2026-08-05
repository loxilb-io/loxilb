# tlsproxyprotov2 — TLS 1.3 + PROXY protocol v2 reproduction

Reproduces loxilb Issue **#1044** / Discussion **#1089**: with a load balancer in
`fullnat` mode and PROXY protocol v2 enabled (`--ppv2en`), the **TLS 1.3 handshake
fails** while TLS 1.2 works.

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
./validation.sh      # prints CONTROL-1/CONTROL-2/REPRO and REPRODUCED/NOT
./rmconfig.sh
```

Knob: `REPRO_MTU=<n> ./validation.sh` to change the forced client PMTU.

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

## CI status

RED until #1044 is fixed (this scenario now exits non-zero when the bug is NOT reproduced,
i.e. after a fix it must be updated to expect success). Not yet wired into a GitHub workflow;
run manually, or promote to a sanity workflow once the eBPF fix lands.
