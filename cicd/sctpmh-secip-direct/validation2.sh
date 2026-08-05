#!/bin/bash
# Issue #1078 — Step 1 verification: an ESTABLISHED association's secondary VIP
# path must become CONFIRMED (and survive primary-path failover).
#
# Unlike validation.sh (which makes short independent connections directly to a
# secondary VIP and only proves Phase 1 reachability), this test reproduces the
# reporter's actual scenario:
#   - uni-homed client establishes ONE association to the PRIMARY VIP,
#   - LoxiLB advertises the secondary VIPs in INIT-ACK,
#   - the client adds them as remote transport addresses and heartbeats them,
#   - those secondary paths must transition UNCONFIRMED(3) -> ACTIVE(2)
#     (per enum sctp_spinfo_state: INACTIVE=0, PF=1, ACTIVE=2, UNCONFIRMED=3).
#
# A dedicated SINGLE-backend rule is used so endpoint-selection consistency
# (Phase 2 "gap A", which only affects multi-backend RR/HASH) does not add
# nondeterminism here. This is exactly the reporter's single-endpoint setup.
source ../common.sh
echo SCENARIO-SCTPMH-SECIP-ASSOC

llb=llb1
pri="50.50.50.1"
sec=( "51.51.51.1" "52.52.52.1" )
ep="10.0.3.10"
port=38412
cport=39412

srv_pid=""
cli_pid=""
cleanup() {
    # Terminate gracefully (SIGTERM) so sctp_darn can restore its tty state;
    # SIGKILL on a process holding the terminal is what corrupts the shell.
    [ -n "$cli_pid" ] && kill "$cli_pid" >/dev/null 2>&1
    [ -n "$srv_pid" ] && kill "$srv_pid" >/dev/null 2>&1
    sudo pkill -f "sctp_darn -H 0.0.0.0 -P $port" >/dev/null 2>&1
    sudo pkill -f "sctp_darn -H 10.0.3.71 -P $cport" >/dev/null 2>&1
    $hexec c1 ip route del blackhole $pri >/dev/null 2>&1
    $dexec $llb loxicmd delete lb $pri --sctp=$port >/dev/null 2>&1
    sudo rm -f v2_*.out >/dev/null 2>&1
}
trap cleanup EXIT

# read the STATE column (last field) of a remote address from the client assoc
remaddr_state() { # $1 = ip
    $hexec c1 cat /proc/net/sctp/remaddr 2>/dev/null | awk -v ip="$1" '$1==ip {print $NF; exit}'
}

# Dedicated single-backend rule with two secondary VIPs (reporter-faithful).
$dexec $llb loxicmd create lb $pri --sctp=${port}:${port} --secips=$(IFS=,; echo "${sec[*]}") --endpoints=${ep}:1 --mode=fullnat
$dexec $llb bash -c 'for i in /proc/sys/net/ipv4/conf/*/rp_filter; do echo 0 > "$i"; done'
sleep 5

# Backend listens (single endpoint); kernel answers heartbeats regardless of app.
# stdin from /dev/null and output to a file so this backgrounded process never
# holds the terminal: a tty-holding process later killed is what leaves the shell
# corrupted (needing `reset`).
$hexec ep1 sctp_darn -H 0.0.0.0 -P $port -l > v2_ep1.out 2>&1 < /dev/null &
srv_pid=$!
disown 2>/dev/null   # drop from the job table so no async "Killed" line is printed

# Uni-homed client keeps one association to the PRIMARY VIP alive by feeding a
# message per second (~180s, outlasting both phases). The feeder subshell reads
# /dev/null, not the tty.
( for i in $(seq 1 180); do echo "m$i"; sleep 1; done ) < /dev/null | \
    $hexec c1 sctp_darn -H 10.0.3.71 -P $cport -h $pri -p $port -s > v2_c1.out 2>&1 &
cli_pid=$!
disown 2>/dev/null

sleep 5

code=0

# --- Phase A: secondary VIP paths must reach ACTIVE(2) ---
echo "--- waiting for secondary VIP paths to become ACTIVE ---"
confA=0
for ((i=0; i<30; i++)); do
    echo "[t=$i] remaddr:"
    $hexec c1 cat /proc/net/sctp/remaddr 2>/dev/null
    allok=1
    for s in "${sec[@]}"; do
        st=$(remaddr_state "$s")
        if [[ "$st" != "2" ]]; then allok=0; fi
        echo "    $s STATE=${st:-none}"
    done
    if [[ $allok == 1 ]]; then confA=1; echo "all secondary paths ACTIVE"; break; fi
    sleep 2
done
if [[ $confA == 1 ]]; then
    echo "Phase A: secondary VIP path confirmation [OK]"
else
    echo "Phase A: secondary VIP path confirmation [NOK]"
    code=1
fi

# --- Phase B: primary-path failover ---
# Blackhole the primary VIP at the client so DATA/HB to it fail. The association
# must survive: primary path leaves ACTIVE and at least one secondary path stays
# ACTIVE (data fails over). Uses the client-internal route so no separate
# physical path is needed in this single-bridge testbed.
if [[ $confA == 1 ]]; then
    echo "--- failover: blackholing primary VIP $pri at client ---"
    $hexec c1 ip route add blackhole $pri
    foOK=0
    for ((i=0; i<30; i++)); do
        echo "[fo t=$i] remaddr:"
        $hexec c1 cat /proc/net/sctp/remaddr 2>/dev/null
        prist=$(remaddr_state "$pri")
        secact=0
        for s in "${sec[@]}"; do
            [[ "$(remaddr_state "$s")" == "2" ]] && secact=1
        done
        # association must still exist (a remaddr row present) and a secondary
        # must remain ACTIVE while the primary is no longer ACTIVE.
        if [[ -n "$prist" && "$prist" != "2" && $secact == 1 ]]; then
            foOK=1; echo "primary left ACTIVE (STATE=$prist), secondary still ACTIVE"; break
        fi
        sleep 2
    done
    if [[ $foOK == 1 ]]; then
        echo "Phase B: primary-path failover [OK]"
    else
        echo "Phase B: primary-path failover [NOK]"
        code=1
    fi
fi

if [[ $code == 0 ]]; then
    echo SCENARIO-SCTPMH-SECIP-ASSOC [OK]
else
    echo SCENARIO-SCTPMH-SECIP-ASSOC [FAILED]
    echo "--- debug: loxicmd get ct ---"
    $dexec $llb loxicmd get ct 2>/dev/null
    echo "--- debug: loxicmd get lb ---"
    $dexec $llb loxicmd get lb -o wide 2>/dev/null
fi
exit $code
