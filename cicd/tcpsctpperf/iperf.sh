#!/bin/bash
count=$1
time=$2

iperf -c 20.20.20.1 -t $time -p 12865 -P $count > iperf.log &
sleep $((time + 20))
res=$(grep SUM iperf.log | tail -1| xargs | cut -d ' ' -f 6)
unit=$(grep SUM iperf.log | tail -1| xargs | cut -d ' ' -f 7)
echo -e "TCP throughput \t\t: $res $unit"
rm -rf iperf.log

resNum=$(bc -l <<<"${res}")
if [[ $resNum < 10 ]]; then
  echo "Failed too low $resNum"
  exit 1
fi

sleep 2

iperf3 -c 20.20.20.1 -t $time -p 13866 -P $count --logfile iperf.log --sctp &
sleep $((time + 2))
res=$(grep SUM iperf.log | tail -1| xargs | cut -d ' ' -f 6)
unit=$(grep SUM iperf.log | tail -1| xargs | cut -d ' ' -f 7)
echo -e "SCTP throughput \t: $res $unit"
rm -rf iperf.log
