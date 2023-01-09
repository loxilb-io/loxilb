#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1 --with-ka out --ka-config $(pwd)/keepalived_config
spawn_docker_host --dock-type loxilb --dock-name llb2 --with-ka out --ka-config $(pwd)/keepalived_config
spawn_docker_host --dock-type host --dock-name ep1
spawn_docker_host --dock-type host --dock-name ep2
spawn_docker_host --dock-type host --dock-name ep3
spawn_docker_host --dock-type host --dock-name r1
spawn_docker_host --dock-type host --dock-name user

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


##Pod networks
config_docker_host --host1 ep1 --host2 r1 --ptype phy --addr 11.11.11.3/24 --gw 11.11.11.11
config_docker_host --host1 ep2 --host2 r1 --ptype phy --addr 11.11.11.4/24 --gw 11.11.11.11
config_docker_host --host1 ep3 --host2 r1 --ptype phy --addr 11.11.11.5/24 --gw 11.11.11.11

$hexec r1 ip route add 20.20.20.1/32 via 11.11.11.11
add_route llb1 1.1.1.0/24 11.11.11.254
add_route llb2 1.1.1.0/24 11.11.11.254

sleep 1

##Create LB rule
create_lb_rule llb1 20.20.20.1 --tcp=2020:8080 --endpoints=11.11.11.3:1,11.11.11.4:1,11.11.11.5:1 --mode=fullnat
create_lb_rule llb2 20.20.20.1 --tcp=2020:8080 --endpoints=11.11.11.3:1,11.11.11.4:1,11.11.11.5:1 --mode=fullnat

# keepalive will take few seconds to be UP and running with valid states
sleep 10
