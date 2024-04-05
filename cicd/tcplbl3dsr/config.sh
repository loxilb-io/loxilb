#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type host --dock-name l3h1
spawn_docker_host --dock-type host --dock-name l3ep1
spawn_docker_host --dock-type host --dock-name l3ep2
spawn_docker_host --dock-type host --dock-name l3ep3

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts l3h1 llb1 8000
connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1

sleep 5

#L3 config
config_docker_host --host1 l3h1 --host2 llb1 --ptype phy --addr 10.10.10.1/24 --gw 10.10.10.254
config_docker_host --host1 l3ep1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 l3ep2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 l3ep3 --host2 llb1 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254
config_docker_host --host1 llb1 --host2 l3h1 --ptype phy --addr 10.10.10.254/24
config_docker_host --host1 llb1 --host2 l3ep1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 l3ep2 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 llb1 --host2 l3ep3 --ptype phy --addr 33.33.33.254/24

#IPinIP Config 
$hexec llb1 ip link add name ipip0 type ipip local 31.31.31.254 remote 31.31.31.1
$hexec llb1 ip link set dev ipip0 up
$hexec llb1 ip addr add 45.45.45.254/24 dev ipip0
$hexec llb1 ip route add 56.56.56.0/24 via 45.45.45.1

$hexec llb1 ip link add name ipip1 type ipip local 32.32.32.254 remote 32.32.32.1
$hexec llb1 ip link set dev ipip1 up
$hexec llb1 ip addr add 46.46.46.254/24 dev ipip1
$hexec llb1 ip route add 57.57.57.0/24 via 46.46.46.1

$hexec llb1 ip link add name ipip2 type ipip local 33.33.33.254 remote 33.33.33.1
$hexec llb1 ip link set dev ipip2 up
$hexec llb1 ip addr add 47.47.47.254/24 dev ipip2
$hexec llb1 ip route add 58.58.58.0/24 via 47.47.47.1

$hexec l3ep1 ip link add name ipip0 type ipip local 31.31.31.1 remote 31.31.31.254
$hexec l3ep1 ip link set dev ipip0 up
$hexec l3ep1 ip addr add 45.45.45.1/24 dev ipip0
#$hexec l3ep1 ip route add 10.10.10.0/24 dev ipip0
$hexec l3ep1 ip addr add 56.56.56.1/32 dev lo
$hexec l3ep1 ip addr add 20.20.20.1/32 dev lo

$hexec l3ep2 ip link add name ipip0 type ipip local 32.32.32.1 remote 32.32.32.254
$hexec l3ep2 ip link set dev ipip0 up
$hexec l3ep2 ip addr add 46.46.46.1/24 dev ipip0
#$hexec l3ep2 ip route add 10.10.10.0/24 dev ipip0
$hexec l3ep2 ip addr add 57.57.57.1/32 dev lo
$hexec l3ep2 ip addr add 20.20.20.1/32 dev lo

$hexec l3ep3 ip link add name ipip0 type ipip local 33.33.33.1 remote 33.33.33.254
$hexec l3ep3 ip link set dev ipip0 up
$hexec l3ep3 ip addr add 47.47.47.1/24 dev ipip0
#$hexec l3ep3 ip route add 10.10.10.0/24 dev ipip0
$hexec l3ep3 ip addr add 58.58.58.1/32 dev lo
$hexec l3ep3 ip addr add 20.20.20.1/32 dev lo

$hexec l3ep1 sysctl net.ipv4.conf.el3ep1llb1.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep2 sysctl net.ipv4.conf.el3ep2llb1.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep3 sysctl net.ipv4.conf.el3ep3llb1.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep1 sysctl net.ipv4.conf.ipip0.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep2 sysctl net.ipv4.conf.ipip0.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep3 sysctl net.ipv4.conf.ipip0.rp_filter=0 2>&1 >> /dev/null

sleep 5
create_lb_rule llb1 20.20.20.1 --select=hash --tcp=8080:8080 --endpoints=56.56.56.1:1,57.57.57.1:1,58.58.58.1:1 --mode=dsr
