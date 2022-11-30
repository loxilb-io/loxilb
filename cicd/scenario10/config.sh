#!/bin/bash
source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type loxilb --dock-name llb2
spawn_docker_host --dock-type host --dock-name h1
spawn_docker_host --dock-type host --dock-name h2
spawn_docker_host --dock-type host --dock-name l3e1
spawn_docker_host --dock-type host --dock-name l3e2
spawn_docker_host --dock-type host --dock-name l3e3

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts h1 llb1
connect_docker_hosts h2 llb1
connect_docker_hosts llb1 llb2

config_docker_host --host1 h1 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 llb1 --host2 h1 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 h2 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 llb1 --host2 h2 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 llb2 --ptype phy --addr 10.10.10.59/24
config_docker_host --host1 llb2 --host2 llb1 --ptype phy --addr 10.10.10.56/24

## VxLAN tunnel between llb1 and llb2
create_docker_host_vxlan --host1 llb1 --host2 llb2 --id 50 --uif phy --lip 10.10.10.59
create_docker_host_vxlan --host1 llb1 --host2 llb2 --id 50 --ep 10.10.10.56
config_docker_host --host1 llb1 --host2 llb2 --ptype vxlan --id 50 --addr 50.50.50.1/24 --gw 50.50.50.2

create_docker_host_vxlan --host1 llb2 --host2 llb1 --id 50 --uif phy --lip 10.10.10.56
create_docker_host_vxlan --host1 llb2 --host2 llb1 --id 50 --ep 10.10.10.59
config_docker_host --host1 llb2 --host2 llb1 --ptype vxlan --id 50 --addr 50.50.50.2/24 --gw 50.50.50.1

#Endpoint Config
connect_docker_hosts l3e1 llb2
connect_docker_hosts l3e2 llb2
connect_docker_hosts l3e3 llb2

config_docker_host --host1 l3e1 --host2 llb2 --ptype phy --addr 25.25.25.1/24 --gw 25.25.25.254
config_docker_host --host1 llb2 --host2 l3e1 --ptype phy --addr 25.25.25.254/24
config_docker_host --host1 l3e2 --host2 llb2 --ptype phy --addr 26.26.26.1/24 --gw 26.26.26.254
config_docker_host --host1 llb2 --host2 l3e2 --ptype phy --addr 26.26.26.254/24
config_docker_host --host1 l3e3 --host2 llb2 --ptype phy --addr 27.27.27.1/24 --gw 27.27.27.254
config_docker_host --host1 llb2 --host2 l3e3 --ptype phy --addr 27.27.27.254/24

$dexec llb1 ip route add 25.25.25.0/24 via 10.10.10.56 dev ellb1llb2
$dexec llb1 ip route add 26.26.26.0/24 via 10.10.10.56 dev ellb1llb2
$dexec llb1 ip route add 27.27.27.0/24 via 10.10.10.56 dev ellb1llb2
$dexec llb1 ip route add 88.88.88.88/32 via 10.10.10.56 dev ellb1llb2

$dexec llb2 ip route add 31.31.31.0/24 via 10.10.10.59 dev ellb2llb1
$dexec llb2 ip route add 32.32.32.0/24 via 10.10.10.59 dev ellb2llb1

##Create LB rule
$dexec llb2 loxicmd create lb 88.88.88.88 --sctp=2020:8080 --endpoints=25.25.25.1:1,26.26.26.1:1,27.27.27.1:1
