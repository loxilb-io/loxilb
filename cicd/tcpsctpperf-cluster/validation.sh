#!/bin/bash
source ../common.sh
if [ -z "$1" ]; then
    threads=50
else
    threads=$1
fi

if [ -z "$2" ]; then
    time=10
else
    time=$2
fi

echo SCENARIO-tcpsctpperf-cluster

echo -e "\n\nIPERF Test - Threads: $threads  Duration: $time"
echo "*********************************************************************"
#$hexec l3ep1 iperf -s -p 12865 2>&1 > /dev/null &
#$hexec l3ep1 iperf3 -s -p 13866 --logfile iperf3s.log 2>&1> /dev/null &
sleep 2
vagrant ssh client -c '/vagrant/iperf.sh $threads $time'
#sudo pkill iperf 2>&1>/dev/null
#sudo rm iperf3s.log
echo "*********************************************************************"
sleep 2

#$hexec l3ep1 ./netserver -4 -p 12865
echo -e "\n\nNETPERF Test - Threads: $threads  Duration: $time"
echo "*********************************************************************"
sleep 2
vagrant ssh client -c '/vagrant/netperf.sh $threads $time'
#sudo pkill netserver

#netserver somehow corrupts /dev/null, so we have to create it again
#sudo rm -f /dev/null; sudo mknod -m 666 /dev/null c 1 3
echo "*********************************************************************"

echo SCENARIO-tcpsctpperf-cluster [OK]
