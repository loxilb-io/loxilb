#!/bin/bash
source ../common.sh
source check_ha.sh

echo -e "sctpmh: SCTP Multihoming Basic Test - Client & EP Uni-homed and LB is Multi-homed\n"
extIP="123.123.123.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

$hexec ep1 sctp_darn -H 0.0.0.0  -P 9999 -l 2>&1> /dev/null &

sleep 10
$hexec user stdbuf -oL sctp_darn -H 1.1.1.1 -h $extIP -p $port -s < input > user.out
sleep 3

exp="New connection, peer addresses
123.123.123.1:2020
124.124.124.1:2020
125.125.125.1:2020"

res=`cat user.out | grep -A 3 "New connection, peer addresses"`
sudo rm -rf user.out
sudo pkill sctp_darn

if [[ "$res" == "$exp" ]]; then
    echo $res
    echo -e "\nsctpmh SCTP Multihoming service Basic Test [OK]\n"
    echo "OK" > status1.txt
    restart_loxilbs
else
    echo "NOK" > status1.txt
    echo "sctpmh SCTP Multihoming service Basic Test [NOK]"
    echo "Expected : $exp"
    echo "Received : $res"
    ## Dump some debug info
    echo "system route-info"
    echo -e "\nuser"
    sudo ip netns exec user ip route
    echo -e "\nr1"
    sudo ip netns exec r1 ip route
    echo -e "\nr2"
    sudo ip netns exec r2 ip route
    echo -e "\nllb1"
    sudo ip netns exec llb1 ip route
    echo -e "\nr3"
    sudo ip netns exec r3 ip route
    echo -e "\nr4"
    sudo ip netns exec r4 ip route
    echo "-----------------------------"

    echo -e "\nllb1 lb-info"
    $dexec llb1 loxicmd get lb
    echo "llb1 ep-info"
    $dexec llb1 loxicmd get ep
    echo "llb1 bpf-info"
    $dexec llb1 tc filter show dev eth0 ingress
    echo "BFP trace -- "
    sudo timeout 5 cat  /sys/kernel/debug/tracing/trace_pipe
    sudo killall -9 cat
    echo "BFP trace -- "
    restart_loxilbs
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
