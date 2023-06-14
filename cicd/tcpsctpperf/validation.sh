#!/bin/bash
source ../common.sh
echo SCENARIO-tcpsctpperf

echo -e "\n\nIPERF Test"
echo "*********************************************************************"
$hexec l3ep1 iperf -s -p 12865 2>&1 > /dev/null &
$hexec l3ep1 iperf3 -s -p 13866 --logfile iperf3s.log 2>&1> /dev/null &
sleep 2
$hexec l3h1 ./iperf.sh 50
sudo pkill iperf 2>&1>/dev/null
sudo rm iperf3s.log
echo "*********************************************************************"
sleep 2

$hexec l3ep1 ./netserver -4 -p 12865
echo -e "\n\nNETPERF Test"
echo "*********************************************************************"
sleep 2
$hexec l3h1 ./netperf.sh 50
sudo pkill netserver

#netserver somehow corrupts /dev/null, so we have to create it again
sudo rm -f /dev/null; sudo mknod -m 666 /dev/null c 1 3
echo "*********************************************************************"

echo SCENARIO-tcpsctpperf [OK]
