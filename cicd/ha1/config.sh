#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host loxilb llb1
spawn_docker_host loxilb llb2
spawn_docker_host host ep1
spawn_docker_host host ep2
spawn_docker_host host ep3
spawn_docker_host host r1
#spawn_docker_host host r2
spawn_docker_host host user

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts user r1
connect_docker_hosts r1 llb1
connect_docker_hosts r1 llb2
connect_docker_hosts r1 ep1
connect_docker_hosts r1 ep2
connect_docker_hosts r1 ep3

#node1 config
config_docker_host --host1 user --host2 r1 --ptype phy --addr 1.1.1.1/24 --gw 1.1.1.254
config_docker_host --host1 r1 --host2 user --ptype phy --addr 1.1.1.254/24

create_docker_host_vlan --host1 r1 --host2 llb1 --id 11 --ptype untagged
create_docker_host_vlan --host1 r1 --host2 llb2 --id 11 --ptype untagged
config_docker_host --host1 r1 --host2 llb1 --ptype vlan --id 11 --addr 11.11.11.254/24

create_docker_host_vlan --host1 llb1 --host2 r1 --id 11 --ptype untagged
config_docker_host --host1 llb1 --host2 r1 --ptype vlan --id 11 --addr 11.11.11.1/24

create_docker_host_vlan --host1 llb2 --host2 r1 --id 11 --ptype untagged
config_docker_host --host1 llb2 --host2 r1 --ptype vlan --id 11 --addr 11.11.11.2/24


create_docker_host_vlan --host1 r1 --host2 ep1 --id 11 --ptype untagged
create_docker_host_vlan --host1 r1 --host2 ep2 --id 11 --ptype untagged
create_docker_host_vlan --host1 r1 --host2 ep3 --id 11 --ptype untagged
#config_docker_host --host1 r2 --host2 llb1 --ptype vlan --id 11 --addr 11.11.11.3/24


##Pod networks
config_docker_host --host1 ep1 --host2 r1 --ptype phy --addr 11.11.11.3/24 --gw 11.11.11.11
config_docker_host --host1 ep2 --host2 r1 --ptype phy --addr 11.11.11.4/24 --gw 11.11.11.11
config_docker_host --host1 ep3 --host2 r1 --ptype phy --addr 11.11.11.5/24 --gw 11.11.11.11

$hexec r1 ip route add 20.20.20.1/32 via 11.11.11.11
$hexec llb1 ip route add 1.1.1.0/24 via 11.11.11.254
$hexec llb2 ip route add 1.1.1.0/24 via 11.11.11.254

$dexec llb1 apt update
$dexec llb2 apt update
$dexec llb1 apt install -y keepalived curl
$dexec llb2 apt install -y keepalived curl
$dexec llb1 ifconfig eth0 0
$dexec llb2 ifconfig eth0 0


docker cp keepalived.conf llb1:/etc/keepalived/
docker cp keepalived.conf llb2:/etc/keepalived/
docker cp notifyha.sh llb1:/root/loxilb-io/loxilb/
docker cp notifyha.sh llb2:/root/loxilb-io/loxilb/
$dexec llb1 systemctl start keepalived.service
$dexec llb2 systemctl start keepalived.service
sleep 1

##Create LB rule
$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=11.11.11.3:1,11.11.11.4:1,11.11.11.5:1 --fullnat
$dexec llb2 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=11.11.11.3:1,11.11.11.4:1,11.11.11.5:1 --fullnat
