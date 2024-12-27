#!/bin/bash
source ../common.sh
echo SCENARIO-tcplbcps

$dexec llb1 /root/llb_cfg_add.sh
sleep 5
echo "loxilb-cps perf"
$hexec l3h1 ./netperf.sh 50
$dexec llb1 /root/llb_cfg_del.sh

sleep 20

echo "ipvs-cps perf"
$dexec llb1 /root/ipvs_cfg_add.sh
sleep 10
$hexec l3h1 ./netperf.sh 50
sleep 5
$dexec llb1 /root/ipvs_cfg_del.sh

echo SCENARIO-tcplbcps [OK]
