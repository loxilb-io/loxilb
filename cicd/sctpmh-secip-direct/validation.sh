#!/bin/bash
# Issue #1078 — verify that a uni-homed client can reach a backend by connecting
# DIRECTLY to a secondary VIP (not just the primary VIP), and that the secondary
# VIP dataplane entry is removed when the rule is deleted.
source ../common.sh
echo SCENARIO-SCTPMH-SECIP-DIRECT

ep=( "10.0.3.10" "10.0.3.11" )
pri="20.20.20.1"
sec=( "21.21.21.1" "22.22.22.1" )

$hexec ep1 ../common/sctp_server ${ep[0]} 38412 server1 >/dev/null 2>&1 &
$hexec ep2 ../common/sctp_server ${ep[1]} 38412 server2 >/dev/null 2>&1 &

sleep 5

code=0

# connect from c1 to $target:38412 and echo "OK"/"NOK" based on whether a valid
# backend ("server1"/"server2") answered.
sctp_try() {
    local target=$1
    local res
    res=$($hexec c1 timeout 10 ../common/sctp_client 10.0.3.71 0 $target 38412)
    if [[ $res == "server1" || $res == "server2" ]]; then
        echo "OK:$res"
    else
        echo "NOK:$res"
    fi
}

# 1) Wait until the service is programmed and backends are up, using the primary
#    VIP (this path has always worked) before exercising the secondary VIPs.
waitCount=0
while true; do
    r=$(sctp_try "$pri")
    if [[ $r == OK:* ]]; then
        echo "Primary VIP $pri reachable (${r#OK:})"
        break
    fi
    waitCount=$(( waitCount + 1 ))
    if [[ $waitCount == 10 ]]; then
        echo "Primary VIP $pri not reachable - aborting"
        echo SCENARIO-SCTPMH-SECIP-DIRECT [FAILED]
        sudo pkill -9 -x sctp_server >/dev/null 2>&1
        exit 1
    fi
    sleep 2
done

# 2) Phase 1 core: connect DIRECTLY to each secondary VIP. Before the fix these
#    miss the NAT map and time out (ECONNREFUSED); after the fix they DNAT to a
#    backend like the primary VIP.
echo "--- direct connect to secondary VIPs ---"
for s in "${sec[@]}"; do
    for i in 1 2 3; do
        r=$(sctp_try "$s")
        if [[ $r == OK:* ]]; then
            echo "secVIP $s -> ${r#OK:} [OK]"
        else
            echo "secVIP $s -> '${r#NOK:}' [NOK]"
            code=1
        fi
        sleep 1
    done
done

# 3) Regression: primary VIP must still work alongside the secondary entries.
echo "--- regression: primary VIP ---"
for i in 1 2 3; do
    r=$(sctp_try "$pri")
    if [[ $r == OK:* ]]; then
        echo "priVIP $pri -> ${r#OK:} [OK]"
    else
        echo "priVIP $pri -> '${r#NOK:}' [NOK]"
        code=1
    fi
    sleep 1
done

# 4) Remove path: delete the rule and confirm the secondary VIP entry is gone
#    (i.e. the per-secondary-VIP NAT entries are cleaned up, not left stale).
echo "--- remove path: delete rule, secondary VIP must stop working ---"
$dexec llb1 loxicmd delete lb $pri --sctp=38412 >/dev/null 2>&1
sleep 3
r=$(sctp_try "${sec[0]}")
if [[ $r == OK:* ]]; then
    echo "secVIP ${sec[0]} still reachable after delete -> stale entry [NOK]"
    code=1
else
    echo "secVIP ${sec[0]} unreachable after delete [OK]"
fi

if [[ $code == 0 ]]; then
    echo SCENARIO-SCTPMH-SECIP-DIRECT [OK]
else
    echo SCENARIO-SCTPMH-SECIP-DIRECT [FAILED]
fi
sudo pkill -9 -x sctp_server >/dev/null 2>&1
exit $code
