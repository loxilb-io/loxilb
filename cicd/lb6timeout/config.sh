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


connect_docker_hosts l3h1 llb1
connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1

$hexec l3h1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null  
$hexec l3h1 sysctl net.ipv6.conf.default.disable_ipv6=0
$hexec l3ep1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec l3ep1 sysctl net.ipv6.conf.default.disable_ipv6=0
$hexec l3ep2 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec l3ep2 sysctl net.ipv6.conf.default.disable_ipv6=0
$hexec l3ep3 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec l3ep3 sysctl net.ipv6.conf.default.disable_ipv6=0
$hexec llb1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec llb1 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec llb1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec llb1 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

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

## IPV6 Stuff
$hexec l3h1 ip -6 addr add 3ffe::1/64 dev el3h1llb1
$hexec l3h1 ip -6 route add default via 3ffe::2

$hexec l3ep1 ip -6 addr add 4ffe::1/64 dev el3ep1llb1
$hexec l3ep1 ip -6 route add default via 4ffe::2

$hexec l3ep2 ip -6 addr add 5ffe::1/64 dev el3ep2llb1
$hexec l3ep2 ip -6 route add default via 5ffe::2

$hexec l3ep3 ip -6 addr add 6ffe::1/64 dev el3ep3llb1
$hexec l3ep3 ip -6 route add default via 6ffe::2

$hexec llb1 ip -6 addr add 3ffe::2/64 dev ellb1l3h1
$hexec llb1 ip -6 addr add 4ffe::2/64 dev ellb1l3ep1
$hexec llb1 ip -6 addr add 5ffe::2/64 dev ellb1l3ep2
$hexec llb1 ip -6 addr add 6ffe::2/64 dev ellb1l3ep3
$hexec llb1 ip -6 addr add 2001::1/128 dev lo

sleep 5
$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --inatimeout=30
$dexec llb1 loxicmd create lb  2001::1 --tcp=2020:8080 --endpoints=4ffe::1:1,5ffe::1:1,6ffe::1:1 --inatimeout=30
