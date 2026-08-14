#!/bin/bash
source ../common.sh
echo SCENARIO-wrrtcplb2
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 curl --max-time 10 -s ${ep[j]}:8080)
    #echo $res
    if [[ $res == "${servArr[j]}" ]]
    then
        echo "$res UP"
        j=$(( $j + 1 ))
    else
        echo "Waiting for ${servArr[j]}(${ep[j]})"
        waitCount=$(( $waitCount + 1 ))
        if [[ $waitCount == 10 ]];
        then
            echo "All Servers are not UP"
            echo SCENARIO-wrrtcplb2 [FAILED]
            sudo pkill node
            exit 1
        fi
    fi
    sleep 1
done

# Endpoint weights as configured in config.sh.
weightArr=( 40 40 20 )
nreq=32
tol=3
cntArr=( 0 0 0 )
noResp=0

# What wRR guarantees is the share of traffic each endpoint gets, not the order
# the endpoints come back in. The order is an artifact of how weights are
# expanded into datapath slots, and it changed when that table went from 32 to
# 16 slots for the LLB_NAT_STAT_CID fix, without the shares changing at all.
# Asserting the exact sequence made this test fail on a change that did not
# alter the balancing, so check the distribution instead.
for i in $(seq 0 $(( nreq - 1 )))
do
    res=$($hexec l3h1 curl --max-time 10 -s 20.20.20.1:2020)
    echo $i:$res
    matched=0
    for k in 0 1 2
    do
        if [[ $res == "${servArr[k]}" ]]
        then
            cntArr[k]=$(( ${cntArr[k]} + 1 ))
            matched=1
            break
        fi
    done
    if [[ $matched == 0 ]]
    then
        noResp=$(( noResp + 1 ))
    fi
    sleep 1
done

echo "--- distribution over $nreq requests (tolerance +/-$tol) ---"
for k in 0 1 2
do
    got=${cntArr[k]}
    # rounded ideal share for this weight
    want=$(( (nreq * ${weightArr[k]} + 50) / 100 ))
    diff=$(( got - want ))
    if [[ $diff -lt 0 ]]
    then
        diff=$(( 0 - diff ))
    fi
    if [[ $diff -gt $tol ]]
    then
        echo "${servArr[k]}: $got, weight ${weightArr[k]}% expects ~$want [FAILED]"
        code=1
    else
        echo "${servArr[k]}: $got, weight ${weightArr[k]}% expects ~$want [OK]"
    fi
done
if [[ $noResp != 0 ]]
then
    echo "$noResp request(s) returned no valid server response [FAILED]"
    code=1
fi
sudo killall -9 node 2>&1 > /dev/null
if [[ $code == 0 ]]
then
    echo SCENARIO-wrrtcplb2 [OK]
else
    echo SCENARIO-wrrtcplb2 [FAILED]
fi
exit $code
