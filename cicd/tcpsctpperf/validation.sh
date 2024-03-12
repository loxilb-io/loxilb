#!/bin/bash
set -eo pipefail
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

echo SCENARIO-tcpsctpperf

echo -e "\n\nIPERF Test - Threads: $threads  Duration: $time"
echo "*********************************************************************"
$hexec l3ep1 iperf -s -p 12865 2>&1 > /dev/null &
$hexec l3ep1 iperf3 -s -p 13866 --logfile iperf3s.log 2>&1> /dev/null &
sleep 10
$hexec l3h1 ./iperf.sh $threads $time
sudo pkill iperf 2>&1>/dev/null
sudo rm iperf3s.log
echo "*********************************************************************"
sleep 2

$hexec l3ep1 ./netserver -4 -p 12865
echo -e "\n\nNETPERF Test - Threads: $threads  Duration: $time"
echo "*********************************************************************"
sleep 10
$hexec l3h1 ./netperf.sh $threads $time
sudo pkill netserver

#netserver somehow corrupts /dev/null, so we have to create it again
sudo rm -f /dev/null; sudo mknod -m 666 /dev/null c 1 3
echo "*********************************************************************"

echo -e "\n\nRestarting loxilb in different mode"
$dexec llb1 pkill -9 loxilb
$dexec llb1 ip link del llb0
$dexec llb1 bash -c "nohup /root/loxilb-io/loxilb/loxilb --rss-enable >> /dev/null 2>&1 &"
sleep 40
for ((i=1,port=12865;i<=100;i++,port++))
do
  $dexec llb1 loxicmd create lb 20.20.20.1 --tcp=$port:$port  --endpoints=31.31.1.1:1 >> /dev/null
done

$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=13866:13866  --endpoints=31.31.1.1:1 >> /dev/null
for ((i=1,port=13866;i<=100;i++,port++))
do
  $dexec llb1 loxicmd create lb 20.20.20.1 --sctp=$port:$port  --endpoints=31.31.1.1:1 >> /dev/null
done

sleep 20

echo -e "\n\nIPERF Test - Threads: $threads  Duration: $time"
echo "*********************************************************************"
$hexec l3ep1 iperf -s -p 12865 2>&1 > /dev/null &
$hexec l3ep1 iperf3 -s -p 13866 --logfile iperf3s.log 2>&1> /dev/null &
sleep 10
$hexec l3h1 ./iperf.sh $threads $time
sudo pkill iperf 2>&1>/dev/null
sudo rm iperf3s.log
echo "*********************************************************************"
sleep 2

$hexec l3ep1 ./netserver -4 -p 12865
echo -e "\n\nNETPERF Test - Threads: $threads  Duration: $time"
echo "*********************************************************************"
sleep 10
$hexec l3h1 ./netperf.sh $threads $time
sudo pkill netserver

#netserver somehow corrupts /dev/null, so we have to create it again
sudo rm -f /dev/null; sudo mknod -m 666 /dev/null c 1 3
echo "*********************************************************************"

echo SCENARIO-tcpsctpperf [OK]
