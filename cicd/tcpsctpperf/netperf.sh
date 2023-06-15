#!/bin/bash
count=$1
time=$2

# TCP Connections per second
for ((i=1,tport=12866;i<=count;i++,tport++))
do
    # Enter the server IP address after -H.
    # Enter the test duration after -l. Set the duration to 10000 to prevent Netperf from ending prematurely.
    # Enter the test method (TCP_RR or TCP_CRR) after -t.
    #./netperf -H xxx.xxx.xxx.xxx -l 10000 -t TCP_CRR -- -r 1,1 &
    #netperf -L 10.10.10.1 -H 20.20.20.1 -t TCP_CRR -4 -p $port -- -P $port > perf$i &
    ./netperf -L 10.10.10.1 -H 20.20.20.1 -t TCP_CRR -l $time -- -P ,$tport > perf$tport &
done
sleep $((time + 10))
ttotal=0

for ((i=1,tport=12866;i<=count;i++,tport++))
do
 a=`awk '/per sec/{getline;getline; print}' perf$tport | xargs | cut -d ' ' -f 6`
 ttotal=`echo "$ttotal + $a" | bc`
done
echo -e "TCP  CPS \t:" $ttotal
rm -fr perf*

# TCP Requests per second
for ((i=1,tport=12866;i<=count;i++,tport++))
do
    ./netperf -L 10.10.10.1 -H 20.20.20.1 -t TCP_RR -l $time -- -P ,$tport > perf$tport &
done
sleep $((time + 10))
ttotal=0

for ((i=1,tport=12866;i<=count;i++,tport++))
do
 a=`awk '/per sec/{getline;getline; print}' perf$tport | xargs | cut -d ' ' -f 6`
 ttotal=`echo "$ttotal + $a" | bc`
done
echo -e "TCP  RPS \t:" $ttotal
rm -fr perf*


# SCTP Requests per second
for ((i=1,sport=13866;i<=count;i++,sport++))
do
    ./netperf -L 10.10.10.1 -H 20.20.20.1 -t SCTP_RR -l $time -- -P ,$sport > perf$sport &
done
sleep $((time + 10))
stotal=0

for ((i=1,sport=13866;i<=count;i++,sport++))
do
 a=`awk '/per sec/{getline;getline; print}' perf$sport | xargs | cut -d ' ' -f 6`
 stotal=`echo "$stotal + $a" | bc`
done
echo -e "SCTP  RPS \t:" $stotal
rm -fr perf*
