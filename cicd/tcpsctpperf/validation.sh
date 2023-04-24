#!/bin/bash
source ../common.sh
echo SCENARIO-tcpsctpperf

echo "loxilb TCP-SCTP RPS perf"
$hexec l3ep1 ./netserver -4 -p 12865
sleep 2
$hexec l3h1 ./netperf.sh 25
sudo pkill netserver >/dev/null 2>&1

echo SCENARIO-tcpsctpperf [OK]
