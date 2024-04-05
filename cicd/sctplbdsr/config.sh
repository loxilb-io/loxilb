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
spawn_docker_host --dock-type host --dock-name sw1

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts l3h1 llb1
connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1
connect_docker_hosts l3h1 sw1
connect_docker_hosts l3ep1 sw1
connect_docker_hosts l3ep2 sw1
connect_docker_hosts l3ep3 sw1

create_docker_host_vlan --host1 l3h1 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 sw1 --host2 l3h1 --id 11 --ptype untagged
config_docker_host --host1 l3h1 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.254/24 --gw 11.11.11.1

create_docker_host_vlan --host1 l3ep1 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 sw1 --host2 l3ep1 --id 11 --ptype untagged
config_docker_host --host1 l3ep1 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.1/24 --gw 11.11.11.254

create_docker_host_vlan --host1 l3ep2 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 sw1 --host2 l3ep2 --id 11 --ptype untagged
config_docker_host --host1 l3ep2 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.2/24 --gw 11.11.11.254

create_docker_host_vlan --host1 l3ep3 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 sw1 --host2 l3ep3 --id 11 --ptype untagged
config_docker_host --host1 l3ep3 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.3/24 --gw 11.11.11.254

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

$hexec l3ep1 ip addr add 20.20.20.1/32 dev lo
$hexec l3ep2 ip addr add 20.20.20.1/32 dev lo
$hexec l3ep3 ip addr add 20.20.20.1/32 dev lo

$hexec l3ep1 ip route add 10.10.10.0/24 via 11.11.11.254
$hexec l3ep2 ip route add 10.10.10.0/24 via 11.11.11.254
$hexec l3ep3 ip route add 10.10.10.0/24 via 11.11.11.254

$hexec l3h1 sysctl net.ipv4.conf.all.rp_filter=0 2>&1 >> /dev/null
$hexec l3h1 sysctl net.ipv4.conf.vlan11.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep1 sysctl net.ipv4.conf.el3ep1llb1.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep2 sysctl net.ipv4.conf.el3ep2llb1.rp_filter=0 2>&1 >> /dev/null
$hexec l3ep3 sysctl net.ipv4.conf.el3ep3llb1.rp_filter=0 2>&1 >> /dev/null

sleep 5
$dexec llb1 loxicmd create lb --select=hash 20.20.20.1 --sctp=2020:2020 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=dsr
