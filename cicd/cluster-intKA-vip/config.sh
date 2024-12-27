#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1 --with-bgp yes --bgp-config $(pwd)/llb1_gobgp_config --with-ka in --ka-config $(pwd)/keepalived_config
spawn_docker_host --dock-type loxilb --dock-name llb2 --with-bgp yes --bgp-config $(pwd)/llb2_gobgp_config --with-ka in --ka-config $(pwd)/keepalived_config
spawn_docker_host --dock-type host --dock-name ep1
spawn_docker_host --dock-type host --dock-name ep2
spawn_docker_host --dock-type host --dock-name ep3
spawn_docker_host --dock-type host --dock-name r1
spawn_docker_host --dock-type host --dock-name r2 --with-bgp yes --bgp-config $(pwd)/quagga_config
spawn_docker_host --dock-type host --dock-name user

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts user r1
connect_docker_hosts r1 llb1
connect_docker_hosts r1 llb2
connect_docker_hosts llb1 r2
connect_docker_hosts llb2 r2
connect_docker_hosts r2 ep1
connect_docker_hosts r2 ep2
connect_docker_hosts r2 ep3

#node1 config
config_docker_host --host1 user --host2 r1 --ptype phy --addr 1.1.1.1/24 --gw 1.1.1.254
config_docker_host --host1 r1 --host2 user --ptype phy --addr 1.1.1.254/24

create_docker_host_vlan --host1 r1 --host2 llb1 --id 11 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 r1 --id 11 --ptype untagged

config_docker_host --host1 r1 --host2 llb1 --ptype vlan --id 11 --addr 11.11.11.254/24 --gw 11.11.11.11
config_docker_host --host1 llb1 --host2 r1 --ptype vlan --id 11 --addr 11.11.11.1/24

create_docker_host_vlan --host1 r1 --host2 llb2 --id 11 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 r1 --id 11 --ptype untagged
config_docker_host --host1 llb2 --host2 r1 --ptype vlan --id 11 --addr 11.11.11.2/24


create_docker_host_vlan --host1 llb1 --host2 r2 --id 10 --ptype untagged
config_docker_host --host1 llb1 --host2 r2 --ptype vlan --id 10 --addr 10.10.10.1/24
create_docker_host_vlan --host1 llb2 --host2 r2 --id 10 --ptype untagged
config_docker_host --host1 llb2 --host2 r2 --ptype vlan --id 10 --addr 10.10.10.2/24

create_docker_host_vlan --host1 r2 --host2 llb1 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 llb2 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep1 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep2 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep3 --id 10 --ptype untagged
config_docker_host --host1 r2 --host2 llb1 --ptype vlan --id 10 --addr 10.10.10.254/24

create_docker_host_vlan --host1 r2 --host2 ep1 --id 31 --ptype untagged
config_docker_host --host1 r2 --host2 ep1 --ptype vlan --id 31 --addr 31.31.31.254/24

create_docker_host_vlan --host1 ep1 --host2 r2 --id 31 --ptype untagged
config_docker_host --host1 ep1 --host2 r2 --ptype vlan --id 31 --addr 31.31.31.1/24 --gw 31.31.31.254

create_docker_host_vlan --host1 r2 --host2 ep2 --id 32 --ptype untagged
config_docker_host --host1 r2 --host2 ep2 --ptype vlan --id 32 --addr 32.32.32.254/24

create_docker_host_vlan --host1 ep2 --host2 r2 --id 32 --ptype untagged
config_docker_host --host1 ep2 --host2 r2 --ptype vlan --id 32 --addr 32.32.32.1/24 --gw 32.32.32.254

create_docker_host_vlan --host1 r2 --host2 ep3 --id 33 --ptype untagged
config_docker_host --host1 r2 --host2 ep3 --ptype vlan --id 33 --addr 33.33.33.254/24

create_docker_host_vlan --host1 ep3 --host2 r2 --id 33 --ptype untagged
config_docker_host --host1 ep3 --host2 r2 --ptype vlan --id 33 --addr 33.33.33.1/24 --gw 33.33.33.254


##Pod networks
$hexec r1 ip route add 20.20.20.1/32 via 11.11.11.11
add_route llb1 1.1.1.0/24 11.11.11.254
add_route llb2 1.1.1.0/24 11.11.11.254

sleep 1
##Create LB rule
create_lb_rule llb1 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=fullnat --bgp
create_lb_rule llb2 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=fullnat --bgp

# keepalive will take few seconds to be UP and running with valid states
sleep 60
