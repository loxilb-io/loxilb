#!/bin/bash
set -eo pipefail

export OSE_LOXILB_SERVERS=${OSE_LOXILB_SERVERS:-1}

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type host --dock-name l3h1
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
  spawn_docker_host --dock-type host --dock-name l3ep$i
done

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts l3h1 llb1
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
  connect_docker_hosts l3ep$i llb1
done

sleep 1

# L3 config
config_docker_host --host1 l3h1 --host2 llb1 --ptype phy --addr 10.10.10.1/24 --gw 10.10.10.254
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
  config_docker_host --host1 l3ep$i --host2 llb1 --ptype phy --addr 31.31.$i.1/24 --gw 31.31.$i.254
done
config_docker_host --host1 llb1 --host2 l3h1 --ptype phy --addr 10.10.10.254/24
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
  config_docker_host --host1 llb1 --host2 l3ep$i --ptype phy --addr 31.31.$i.254/24
done

sleep 1

# Need to do this as netperf sctp doesn't work without this
$hexec l3h1 ifconfig eth0 0
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
  $hexec l3ep$i ifconfig eth0 0
done

for ((i=1,port=12865;i<=100;i++,port++))
do
  $dexec llb1 loxicmd create lb 20.20.20.1 --tcp=$port:$port --endpoints=31.31.1.1:1 >> /dev/null
done

# iperf3 --sctp will use tcp:13866 for control data, and sctp:13866 for the
# benchmark data.
$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=13866:13866 --endpoints=31.31.1.1:1 >> /dev/null
for ((i=1,port=13866;i<=100;i++,port++))
do
  $dexec llb1 loxicmd create lb 20.20.20.1 --sctp=$port:$port --endpoints=31.31.1.1:1 >> /dev/null
done

$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=14000:14000 --endpoints=$(seq --sep , --format '31.31.%g.1:1' 1 $OSE_LOXILB_SERVERS) >> /dev/null
