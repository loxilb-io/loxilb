#!/bin/bash
count=$1
for ((i=1,port=12866;i<=count;i++,port++))
do
    # Enter the server IP address after -H.
    # Enter the test duration after -l. Set the duration to 10000 to prevent Netperf from ending prematurely.
    # Enter the test method (TCP_RR or TCP_CRR) after -t.
    #./netperf -H xxx.xxx.xxx.xxx -l 10000 -t TCP_CRR -- -r 1,1 &
    #netperf -L 10.10.10.1 -H 20.20.20.1 -t TCP_CRR -4 -p $port -- -P $port > perf$i &
    netperf -L 10.10.10.1 -H 20.20.20.1 -t TCP_CRR -- -P ,$port > perf$i &
done
sleep 15
total=0
for ((i=1;i<=count;i++))
do
 a=`awk '/per sec/{getline;getline; print}' perf$i | xargs | cut -d ' ' -f 6`
 total=`echo "$total + $a" | bc`
done
echo $total
rm -fr perf*
