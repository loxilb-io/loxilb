#!/bin/bash

source ../common.sh

./rmconfig.sh

sleep 5

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host loxilb llb1
spawn_docker_host host n1p1
spawn_docker_host host n1p2
spawn_docker_host host n2p1
spawn_docker_host host n3p1
spawn_docker_host host k8n1

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts n1p1 llb1
connect_docker_hosts n1p2 llb1
connect_docker_hosts k8n1 llb1
connect_docker_hosts n2p1 k8n1
connect_docker_hosts n2p2 k8n1

#node1 config
config_docker_host --host1 n1p1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 n1p2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 llb1 --host2 n1p1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 n1p2 --ptype phy --addr 32.32.32.254/24

#node2 config
config_docker_host --host1 n2p1 --host2 k8n1 --ptype phy --addr 5.5.5.2/24
create_docker_host_vxlan --host1 n2p1 --host2 k8n1 --id 60 --uif phy --lip 5.5.5.2
config_docker_host --host1 n2p1 --host2 k8n1 --ptype vxlan --id 60 --addr 5.5.5.2/24 --gw 60.60.60.254
create_docker_host_vxlan --host1 n2p1 --host2 k8n1 --id 60 --ep 5.5.5.1

#Loxilb config
create_docker_host_vlan --host1 llb1 --host2 k8n1 --id 60 --ptype untagged

config_docker_host --host1 llb1 --host2 k8n1 --ptype phy --addr 5.5.5.1/24
create_docker_host_vxlan --host1 llb1 --host2 k8n1 --id 60 --uif phy --lip 5.5.5.1 --pvid 60
create_docker_host_vxlan --host1 llb1 --host2 n2p1 --id 60 --ep 5.5.5.2
config_docker_host --host1 llb1 --host2 n2p1 --ptype vlan --id 60 --addr 60.60.60.254/24
