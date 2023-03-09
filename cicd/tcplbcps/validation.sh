#!/bin/bash
source ../common.sh
echo SCENARIO-tcplbcps
$hexec l3h1 ./netperf.sh 50
echo SCENARIO-tcplbcps [OK]
