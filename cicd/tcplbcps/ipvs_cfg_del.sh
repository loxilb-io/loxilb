#!/bin/bash

for ((i=1,port=12865;i<=150;i++,port++))
do
  ipvsadm -D -t 20.20.20.1:$port;
done

ip addr del  20.20.20.1/32 dev lo
ip link del llb0

nohup /root/loxilb-io/loxilb/loxilb >> /dev/null 2>&1 &
