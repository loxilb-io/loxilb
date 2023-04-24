#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1 
spawn_docker_host --dock-type host --dock-name ep1
spawn_docker_host --dock-type host --dock-name ep2
spawn_docker_host --dock-type host --dock-name c1
spawn_docker_host --dock-type host --dock-name br1

##Create LB rule
create_lb_rule llb1 123.123.123.1 --sctp=38412:38412 --endpoints=10.75.188.218:1,10.75.188.220:1 --mode=onearm

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts llb1 c1
connect_docker_hosts llb1 br1
connect_docker_hosts ep1  br1
connect_docker_hosts ep2  br1

config_docker_host --host1 llb1 --host2 br1 --ptype phy --addr 10.75.188.224/24
config_docker_host --host1 ep1 --host2 br1 --ptype phy --addr 10.75.188.218/24 --gw 10.75.188.224
config_docker_host --host1 ep2 --host2 br1 --ptype phy --addr 10.75.188.220/24 --gw 10.75.188.224
config_docker_host --host1 c1 --host2 llb1 --ptype phy --addr 10.75.191.224/24 --gw 10.75.191.113
config_docker_host --host1 llb1 --host2 c1 --ptype phy --addr 10.75.191.113/24 

sleep 1

create_docker_host_cnbridge --host1 br1 --host2 llb1
create_docker_host_cnbridge --host1 br1 --host2 ep1
create_docker_host_cnbridge --host1 br1 --host2 ep2

$hexec c1 ethtool --offload  ec1llb1 rx off tx off
$hexec ep1 ethtool --offload  eep1br1 rx off tx off
$hexec ep2 ethtool --offload  eep2br1 rx off tx off
$hexec llb1 ethtool --offload ellb1c1 rx off tx off
$hexec llb1 ethtool --offload ellb1br1 rx off tx off

$dexec llb1 bash -c 'for i in /proc/sys/net/ipv4/conf/*/rp_filter; do echo 0 > "$i"; done'

sleep 10
