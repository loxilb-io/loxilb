#!/bin/bash

# Reproduction of Issue #1044 / Discussion #1089 and Issue #675:
#   TLS 1.3 handshake fails when PROXY protocol v2 is enabled.
#
# Three independent defects in the ppv2 datapath are exercised here:
#   REPRO   - the ClientHello arrives as a GSO super-packet (#1044/#1089)
#   REPRO-2 - the client sends no TCP options, i.e. doff == 20 (#675, Windows)
#   REPRO-3 - the L4 checksum is finalised in software downstream (#675, the
#             reason --ppv2en only worked with checksum offload switched off)
#
# ROOT CAUSE (reproduced here, pure docker/veth): the eBPF dp_ins_ppv2() inserts
# the 28-byte PROXY v2 header into the first TCP data packet (the TLS ClientHello)
# using bpf_skb_adjust_room(..., BPF_F_ADJ_ROOM_FIXED_GSO). When that packet is a
# GSO super-packet (gso_size>0) the insertion corrupts the byte stream, so the
# backend cannot parse the PROXY header / TLS record -> handshake dies right after
# ClientHello.  See ../../docs/tls13-proxyproto-v2-issue/.
#
# The baseline scenario never reproduced this because at the default MTU the
# ClientHello fits in ONE segment (< MSS) => not a GSO skb => the FIXED_GSO path is
# never taken. This script FORCES the GSO path in veth with two knobs:
#   1. Shrink the client's send-MSS to the VIP (small per-route PMTU) AND
#   2. Inflate the ClientHello past that MSS (long SNI),
# so the client's GSO builds a gso_size>0 super-packet for the hello.
#
# The control is decisive: with client segmentation-offload OFF (real separate
# segments, same small MSS) the handshake SUCCEEDS; only with offload ON (GSO
# super-packet) does it FAIL. That rules out "small MTU / PMTU" as the cause and
# pins it on the GSO super-packet handling in dp_ins_ppv2.

source ../common.sh
echo SCENARIO-tlsproxyprotov2

vip="10.10.10.254"; vport="2020"; servers="server1 server2 server3"
CLIENT_IF="el3h1llb1"; LLB_CLIENT_IF="ellb1l3h1"; LLB_EP1_IF="ellb1l3ep1"; EP_IF="el3ep1llb1"

# Long SNI inflates the TLS 1.3 ClientHello (~500B) past the small MSS below.
SNI="$(head -c 180 </dev/zero | tr '\0' a).loxilb.io"
REPRO_MTU="${REPRO_MTU:-320}"     # client->VIP PMTU: MSS ~256, hello ~500 => GSO skb
ERR_RE='tlsv1 alert protocol version|alert number 70|:0A00042E:'

setmtu() { $hexec l3h1 ip route del ${vip}/32 2>/dev/null; [[ -n "$1" ]] && $hexec l3h1 ip route replace ${vip}/32 dev $CLIENT_IF mtu $1 2>/dev/null; }
setoff() { # $1=on|off on the whole client<->loxilb<->backend path
  $hexec l3h1  ethtool -K $CLIENT_IF     tso $1 gso $1 gro $1 2>/dev/null
  $hexec llb1  ethtool -K $LLB_CLIENT_IF tso $1 gso $1 gro $1 2>/dev/null
  $hexec llb1  ethtool -K $LLB_EP1_IF    tso $1 gso $1 gro $1 2>/dev/null
  $hexec l3ep1 ethtool -K $EP_IF         tso $1 gso $1 gro $1 2>/dev/null
}
setts() { # $1=1|0 -> client TCP timestamp option on/off (controls TCP data offset)
  $hexec l3h1 sysctl -qw net.ipv4.tcp_timestamps=$1 2>/dev/null
}
setllbtxcsum() { # $1=on|off -> TX checksum offload on loxilb's own veths
  $hexec llb1 ethtool -K $LLB_CLIENT_IF tx $1 >/dev/null 2>&1
  $hexec llb1 ethtool -K $LLB_EP1_IF    tx $1 >/dev/null 2>&1
}
is_server() { for s in $servers; do [[ "$1" == "$s" ]] && return 0; done; return 1; }

# openssl s_client probe -> prints raw output. $1 = -tls1_3|-tls1_2
oprobe() { $hexec l3h1 bash -c "timeout 10 openssl s_client $1 -servername '$SNI' -connect ${vip}:${vport} </dev/null 2>&1"; }
# returns OK|ALERT|FAIL
oclass() {
  echo "$1" | grep -qiE "$ERR_RE" && { echo ALERT; return; }
  echo "$1" | grep -qiE "New, TLSv1|handshake has read [1-9]" && { echo OK; return; }
  echo FAIL   # empty output => stalled then closed by backend
}

echo "SNI len=${#SNI}  repro MTU=${REPRO_MTU}"
echo "ethtool on client? $($hexec l3h1 sh -c 'command -v ethtool >/dev/null && echo yes || echo NO')"

# ---------- CONTROL 1: default path (single-segment hello) must work ----------
echo "=================================================================="
echo "CONTROL-1  default MTU, offload ON  (hello is single-segment)"
echo "=================================================================="
setoff on; setmtu ""
c12=$(oprobe -tls1_2); c13=$(oprobe -tls1_3)
echo "  TLS1.2 -> $(oclass "$c12")   TLS1.3 -> $(oclass "$c13")"
base_ok=0; [[ "$(oclass "$c13")" == OK ]] && base_ok=1

# ---------- CONTROL 2: small MSS + multi-segment, offload OFF -> should WORK ----
echo "=================================================================="
echo "CONTROL-2  MTU=$REPRO_MTU, offload OFF  (real multi-segment, NOT GSO)"
echo "=================================================================="
setoff off; setmtu $REPRO_MTU
off_out=$(oprobe -tls1_3); off_cls=$(oclass "$off_out")
echo "  TLS1.3 -> $off_cls   $(echo "$off_out" | grep -oiE 'handshake has read [0-9]+ bytes' | head -1)"

# ---------- REPRO: same MTU, offload ON -> GSO super-packet -> should FAIL ------
echo "=================================================================="
echo "REPRO      MTU=$REPRO_MTU, offload ON   (GSO super-packet ClientHello)"
echo "=================================================================="
setoff on; setmtu $REPRO_MTU
on_out=$(oprobe -tls1_3); on_cls=$(oclass "$on_out")
echo "  openssl TLS1.3 -> $on_cls"
[[ "$on_cls" == FAIL ]] && echo "     (client stalled/closed right after ClientHello -> SSL_ERROR_SYSCALL form, #1089)"
[[ "$on_cls" == ALERT ]] && { echo "     >>> exact #1044 alert observed:"; echo "$on_out" | grep -iE "$ERR_RE|read .*bytes|written .*bytes" | sed 's/^/       /'; }
# NOTE: a plain `curl https://vip` is NOT a good probe here - curl sends no SNI to
# an IP literal, so its ClientHello stays small (< MSS) and single-segment, i.e. it
# does NOT trigger the GSO path. openssl with the long -servername above is what
# inflates the hello past the MSS. Use openssl to reproduce.

# ---------- REPRO-2: no TCP options (doff == 20) -> issue #675 ---------------
# Nothing GSO/MTU related here: default MTU, offload ON, single-segment hello -
# i.e. exactly CONTROL-1 except that the client sends NO TCP timestamp option,
# so every segment carries a bare 20-byte TCP header (doff == 20).
# dp_ins_ppv2() dispatches the ppv2 write on the TCP options length and used to
# have no doff == 20 arm, so the 28-byte hole it opened was left holding
# skb_push() leftovers (stale IP/TCP header bytes) instead of the PROXY header,
# with those bytes missing from the TCP checksum too.
# That is the Windows-only failure of issue #675: Windows/Windows-11 disable TCP
# timestamps by default (doff == 20) while Linux/macOS/Android enable them
# (doff == 32) - which is also why curl under WSL worked from a Windows box that
# failed natively.
echo "=================================================================="
echo "REPRO-2    default MTU, offload ON, client TCP timestamps OFF (doff=20)"
echo "=================================================================="
setoff on; setmtu ""; setts 0
echo "  client net.ipv4.tcp_timestamps=$($hexec l3h1 sysctl -n net.ipv4.tcp_timestamps 2>/dev/null)"
noopt_out=$(oprobe -tls1_3); noopt_cls=$(oclass "$noopt_out")
echo "  TLS1.3 -> $noopt_cls"
[[ "$noopt_cls" != OK ]] && echo "     (backend gets garbage where the PROXY v2 header should be -> #675)"
setts 1

# ---------- REPRO-3: checksum offload on the ppv2 datapath --------------------
# Default MTU, offload ON, timestamps ON - CONTROL-1 again except that loxilb's
# own veths stop advertising TX checksum offload, so the stack has to finalise
# the L4 checksum in software on the way out (skb_checksum_help). That is what
# any tun device does anyway, and it is the only thing that makes this testbed
# check TCP checksums at all: veth-to-veth normally passes CHECKSUM_PARTIAL
# through untouched, so a wrong checksum is never noticed.
#
# The ppv2 datapath used to maintain tcp->check by hand. seq/ack_seq live
# inside the L4 checksum's own coverage, so the finalisation pass already
# accounts for them and the hand-folded delta was counted twice - every packet
# left exactly the ppv2 header length off. dp_ins_ppv2()'s
# bpf_skb_adjust_room() additionally resets the skb to CHECKSUM_NONE and then
# ships the pseudo-header sum as if it were a finished checksum. Plain fullnat
# is unaffected (it uses bpf_l4_csum_replace()), which is what makes --ppv2en
# the only variable here.
echo "=================================================================="
echo "REPRO-3    default MTU, offload ON, loxilb TX csum offload OFF"
echo "=================================================================="
setoff on; setmtu ""; setts 1; setllbtxcsum off
echo "  loxilb veth tx-checksumming: $($hexec llb1 ethtool -k $LLB_CLIENT_IF 2>/dev/null | grep -c 'tx-checksumming: off') of 1 off"
csum_out=$(oprobe -tls1_3); csum_cls=$(oclass "$csum_out")
echo "  TLS1.3 -> $csum_cls"
if [[ "$csum_cls" != OK ]] && command -v tcpdump >/dev/null 2>&1; then
  timeout 8 $hexec l3h1 tcpdump -i $CLIENT_IF -nn -vv "tcp src port ${vport}" >/tmp/ppv2csum.$$ 2>&1 &
  sleep 1; oprobe -tls1_3 >/dev/null 2>&1; wait
  echo "     backend->client packets with a bad checksum: $(grep -c incorrect /tmp/ppv2csum.$$ 2>/dev/null)/$(grep -c cksum /tmp/ppv2csum.$$ 2>/dev/null)"
  rm -f /tmp/ppv2csum.$$
fi
[[ "$csum_cls" != OK ]] && echo "     (loxilb emits ppv2 packets whose L4 checksum a real client rejects)"
setllbtxcsum on

# ---------- backend evidence ----------
echo "------------------------------------------------------------------"
echo "backend nginx PROXY-parse errors for client connections (proof the bytes"
echo "reached the backend but the inserted PROXY header was corrupt):"
echo "  (loxilb's own endpoint health probes also log 'broken header' from the"
echo "   3x.3x.3x.254 gateway addresses - those are expected, not failures)"
for ep in l3ep1 l3ep2 l3ep3; do
  $dexec $ep tail -n 20 /var/log/nginx/error.log 2>/dev/null \
    | grep -i "while reading PROXY protocol" | grep "client: ${vip}" \
    | tail -2 | sed "s/^/  [$ep] /"
done

# ---------- verdict ----------
# EXPECT=fixed (default): regression gate - every case above must pass.
# EXPECT=bug            : assert the bugs still REPRODUCE (run against a pre-fix
#                         image, e.g. LOXILB_IMAGE=ghcr.io/loxilb-io/loxilb:v0.9.8.8).
EXPECT="${EXPECT:-fixed}"
setoff on; setmtu ""; setts 1; setllbtxcsum on
echo "=================================================================="
verdict=1
if [[ $base_ok != 1 ]]; then
  echo "RESULT: inconclusive - even the single-segment control failed (env/routing issue)."
elif [[ "$EXPECT" == "fixed" ]]; then
  if [[ "$on_cls" == OK && "$off_cls" == OK && "$noopt_cls" == OK && "$csum_cls" == OK ]]; then
    echo "RESULT: FIXED - all three ppv2 datapath defects are gone."
    echo "  - single-segment hello (CONTROL-1):        OK"
    echo "  - multi-segment, offload OFF (CONTROL-2):  OK"
    echo "  - GSO super-packet, offload ON  (REPRO):   OK   <- #1044/#1089"
    echo "  - no TCP options, doff=20      (REPRO-2):  OK   <- #675 (Windows)"
    echo "  - L4 csum under offload        (REPRO-3):  OK   <- #675 (offload-off workaround)"
    verdict=0
  else
    echo "RESULT: NOT fixed. gso=$on_cls noopt=$noopt_cls csum=$csum_cls off=$off_cls base_ok=$base_ok"
    [[ "$on_cls"    != OK ]] && echo "  - GSO super-packet still broken   -> dp_ins_ppv2 FIXED_GSO corruption (#1044/#1089)"
    [[ "$noopt_cls" != OK ]] && echo "  - doff=20 still broken            -> dp_ins_ppv2 missing 'no TCP options' arm (#675)"
    [[ "$csum_cls"  != OK ]] && echo "  - checksums still broken          -> ppv2 maintains tcp->check by hand (#675)"
  fi
elif [[ "$on_cls" != OK || "$noopt_cls" != OK || "$csum_cls" != OK ]]; then
  echo "RESULT: REPRODUCED."
  echo "  - single-segment hello (CONTROL-1):        OK"
  echo "  - multi-segment, offload OFF (CONTROL-2):  $off_cls   <- not a PMTU/MTU problem"
  echo "  - GSO super-packet, offload ON  (REPRO):   $on_cls   <- #1044/#1089, dp_ins_ppv2 FIXED_GSO corruption"
  echo "  - no TCP options, doff=20      (REPRO-2):  $noopt_cls   <- #675, dp_ins_ppv2 writes no header at doff=20"
  echo "  - L4 csum under offload        (REPRO-3):  $csum_cls   <- #675, ppv2 maintains tcp->check by hand"
  verdict=0
else
  echo "RESULT: NOT reproduced (no case failed)."
  echo "        gso=$on_cls noopt=$noopt_cls csum=$csum_cls off=$off_cls"
fi
echo "=================================================================="

if [[ $verdict == 0 ]]; then
  [[ "$EXPECT" == "fixed" ]] && echo SCENARIO-tlsproxyprotov2 [OK] "(fix verified)" || echo SCENARIO-tlsproxyprotov2 [OK] "(bug reproduced)"
else
  [[ "$EXPECT" == "fixed" ]] && echo SCENARIO-tlsproxyprotov2 [FAILED] "(fix not verified)" || echo SCENARIO-tlsproxyprotov2 [FAILED] "(not reproduced)"
fi
exit $verdict
