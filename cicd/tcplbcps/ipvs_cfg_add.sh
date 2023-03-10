#!/bin/bash

pkill -9 loxilb
for iface in $(ifconfig | cut -d ' ' -f1| tr ':' '\n' | awk NF)
do
  ntc filter del dev $iface ingress >> /dev/null 2>&1;
done

ip addr add 20.20.20.1/32 dev lo

for ((i=1,port=12865;i<=150;i++,port++))
do
  ipvsadm -A -t 20.20.20.1:$port -s rr; ipvsadm -a -t 20.20.20.1:$port -r 31.31.31.1:$port -m; done

