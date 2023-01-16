#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type host --dock-name n1p1
spawn_docker_host --dock-type host --dock-name n1p2
spawn_docker_host --dock-type host --dock-name n2p1
spawn_docker_host --dock-type host --dock-name n3p1
spawn_docker_host --dock-type host --dock-name k8n1

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts n1p1 llb1
connect_docker_hosts n1p2 llb1
connect_docker_hosts k8n1 llb1
connect_docker_hosts n2p1 k8n1
connect_docker_hosts n3p1 k8n1

#node1 config
config_docker_host --host1 n1p1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 n1p2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 llb1 --host2 n1p1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 n1p2 --ptype phy --addr 32.32.32.254/24

#node2 config
config_docker_host --host1 n2p1 --host2 k8n1 --ptype phy --addr 5.5.5.2/24
create_docker_host_vxlan --host1 n2p1 --host2 k8n1 --id 60 --uif phy --lip 5.5.5.2
config_docker_host --host1 n2p1 --host2 k8n1 --ptype vxlan --id 60 --addr 60.60.60.2/24 --gw 60.60.60.254
create_docker_host_vxlan --host1 n2p1 --host2 k8n1 --id 60 --ep 5.5.5.1

#node3 config
config_docker_host --host1 n3p1 --host2 k8n1 --ptype phy --addr 5.5.5.3/24
create_docker_host_vxlan --host1 n3p1 --host2 k8n1 --id 60 --uif phy --lip 5.5.5.3
config_docker_host --host1 n3p1 --host2 k8n1 --ptype vxlan --id 60 --addr 60.60.60.3/24 --gw 60.60.60.254
create_docker_host_vxlan --host1 n3p1 --host2 k8n1 --id 60 --ep 5.5.5.1

#Loxilb config
config_docker_host --host1 llb1 --host2 k8n1 --ptype phy --addr 5.5.5.1/24
create_docker_host_vxlan --host1 llb1 --host2 k8n1 --id 60 --uif phy --lip 5.5.5.1
config_docker_host --host1 llb1 --host2 k8n1 --ptype vxlan --id 60 --addr 60.60.60.254/24 --gw 60.60.60.1
create_docker_host_vxlan --host1 llb1 --host2 k8n1 --id 60 --ep 5.5.5.2
create_docker_host_vxlan --host1 llb1 --host2 k8n1 --id 60 --ep 5.5.5.3

#K8snet config
create_docker_host_cnbridge --host1 k8n1 --host2 llb1
create_docker_host_cnbridge --host1 k8n1 --host2 n2p1
create_docker_host_cnbridge --host1 k8n1 --host2 n3p1

##Pod networks
$hexec n2p1 ip addr add 33.33.33.1/24 dev vxlan60
$hexec n3p1 ip addr add 34.34.34.1/24 dev vxlan60
$hexec llb1 ip route add 33.33.33.1/32 via 60.60.60.2
$hexec llb1 ip route add 34.34.34.1/32 via 60.60.60.3

##Create LB rule
$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,33.33.33.1:1,34.34.34.1:1
