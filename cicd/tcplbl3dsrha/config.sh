#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type host --dock-name r1 --with-bgp yes --bgp-config $(pwd)/quagga_config1

spawn_docker_host --dock-type loxilb --dock-name llb1 --with-bgp yes --bgp-config $(pwd)/llb1_gobgp_config
spawn_docker_host --dock-type loxilb --dock-name llb2 --with-bgp yes --bgp-config $(pwd)/llb2_gobgp_config
spawn_docker_host --dock-type host --dock-name ep1 
spawn_docker_host --dock-type host --dock-name ep2 
spawn_docker_host --dock-type host --dock-name ep3
spawn_docker_host --dock-type host --dock-name r2
spawn_docker_host --dock-type host --dock-name user

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts user r1 8000
connect_docker_hosts r1 llb1
connect_docker_hosts r1 llb2
connect_docker_hosts llb1 ep1
connect_docker_hosts llb1 ep2
connect_docker_hosts llb1 ep3
connect_docker_hosts llb2 ep1
connect_docker_hosts llb2 ep2
connect_docker_hosts llb2 ep3
connect_docker_hosts ep1 r2
connect_docker_hosts ep2 r2
connect_docker_hosts ep3 r2
connect_docker_hosts r1 r2

#node1 config
config_docker_host --host1 user --host2 r1 --ptype phy --addr 1.1.1.1/24 --gw 1.1.1.254
config_docker_host --host1 r1 --host2 user --ptype phy --addr 1.1.1.254/24

config_docker_host --host1 r1 --host2 r2 --ptype phy --addr 2.2.2.1/24
config_docker_host --host1 r2 --host2 r1 --ptype phy --addr 2.2.2.2/24 --gw 2.2.2.1

create_docker_host_vlan --host1 r1 --host2 llb1 --id 11 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 r1 --id 11 --ptype untagged

config_docker_host --host1 r1 --host2 llb1 --ptype vlan --id 11 --addr 11.11.11.254/24
config_docker_host --host1 llb1 --host2 r1 --ptype vlan --id 11 --addr 11.11.11.1/24

create_docker_host_vlan --host1 r1 --host2 llb2 --id 11 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 r1 --id 11 --ptype untagged
config_docker_host --host1 llb2 --host2 r1 --ptype vlan --id 11 --addr 11.11.11.2/24


create_docker_host_vlan --host1 llb1 --host2 ep1 --id 31 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 ep2 --id 32 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 ep3 --id 33 --ptype untagged
config_docker_host --host1 llb1 --host2 ep1 --ptype vlan --id 31 --addr 31.31.31.253/24
config_docker_host --host1 llb1 --host2 ep1 --ptype vlan --id 32 --addr 32.32.32.253/24
config_docker_host --host1 llb1 --host2 ep1 --ptype vlan --id 33 --addr 33.33.33.253/24

create_docker_host_vlan --host1 llb2 --host2 ep1 --id 31 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 ep2 --id 32 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 ep3 --id 33 --ptype untagged
config_docker_host --host1 llb2 --host2 ep1 --ptype vlan --id 31 --addr 31.31.31.254/24
config_docker_host --host1 llb2 --host2 ep1 --ptype vlan --id 32 --addr 32.32.32.254/24
config_docker_host --host1 llb2 --host2 ep1 --ptype vlan --id 33 --addr 33.33.33.254/24

create_docker_host_vlan --host1 ep1 --host2 llb1 --id 31 --ptype untagged
create_docker_host_vlan --host1 ep1 --host2 llb2 --id 31 --ptype untagged
config_docker_host --host1 ep1 --host2 llb1 --ptype vlan --id 31 --addr 31.31.31.1/24

create_docker_host_vlan --host1 ep2 --host2 llb1 --id 32 --ptype untagged
create_docker_host_vlan --host1 ep2 --host2 llb2 --id 32 --ptype untagged
config_docker_host --host1 ep2 --host2 r2 --ptype vlan --id 32 --addr 32.32.32.1/24

create_docker_host_vlan --host1 ep3 --host2 llb1 --id 33 --ptype untagged
create_docker_host_vlan --host1 ep3 --host2 llb2 --id 33 --ptype untagged
config_docker_host --host1 ep3 --host2 r2 --ptype vlan --id 33 --addr 33.33.33.1/24

create_docker_host_vlan --host1 r2 --host2 ep1 --id 3 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep2 --id 3 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep3 --id 3 --ptype untagged
config_docker_host --host1 r2 --host2 ep1 --ptype vlan --id 3 --addr 3.3.3.254/24

create_docker_host_vlan --host1 ep1 --host2 r2 --id 3 --ptype untagged
config_docker_host --host1 ep1 --host2 r2 --ptype vlan --id 3 --addr 3.3.3.1/24 --gw 3.3.3.254

create_docker_host_vlan --host1 ep2 --host2 r2 --id 3 --ptype untagged
config_docker_host --host1 ep2 --host2 r2 --ptype vlan --id 3 --addr 3.3.3.2/24 --gw 3.3.3.254

create_docker_host_vlan --host1 ep3 --host2 r2 --id 3 --ptype untagged
config_docker_host --host1 ep3 --host2 r2 --ptype vlan --id 3 --addr 3.3.3.3/24 --gw 3.3.3.254

#IPinIP Config 
$hexec llb1 ip link add name ipip11 type ipip local 31.31.31.253 remote 31.31.31.1
$hexec llb1 ip link set dev ipip11 up
$hexec llb1 ip addr add 45.45.1.254/24 dev ipip11
$hexec llb1 ip route add 56.56.56.0/24 via 45.45.1.1

$hexec llb1 ip link add name ipip12 type ipip local 32.32.32.253 remote 32.32.32.1
$hexec llb1 ip link set dev ipip12 up
$hexec llb1 ip addr add 46.46.1.254/24 dev ipip12
$hexec llb1 ip route add 57.57.57.0/24 via 46.46.1.1

$hexec llb1 ip link add name ipip13 type ipip local 33.33.33.253 remote 33.33.33.1
$hexec llb1 ip link set dev ipip13 up
$hexec llb1 ip addr add 47.47.1.254/24 dev ipip13
$hexec llb1 ip route add 58.58.58.0/24 via 47.47.1.1

$hexec llb2 ip link add name ipip21 type ipip local 31.31.31.254 remote 31.31.31.1
$hexec llb2 ip link set dev ipip21 up
$hexec llb2 ip addr add 45.45.2.254/24 dev ipip21
$hexec llb2 ip route add 56.56.56.0/24 via 45.45.2.1

$hexec llb2 ip link add name ipip22 type ipip local 32.32.32.254 remote 32.32.32.1
$hexec llb2 ip link set dev ipip22 up
$hexec llb2 ip addr add 46.46.2.254/24 dev ipip22
$hexec llb2 ip route add 57.57.57.0/24 via 46.46.2.1

$hexec llb2 ip link add name ipip23 type ipip local 33.33.33.254 remote 33.33.33.1
$hexec llb2 ip link set dev ipip23 up
$hexec llb2 ip addr add 47.47.2.254/24 dev ipip23
$hexec llb2 ip route add 58.58.58.0/24 via 47.47.2.1

$hexec ep1 ip link add name ipip11 type ipip local 31.31.31.1 remote 31.31.31.253
$hexec ep1 ip link set dev ipip11 up
$hexec ep1 ip addr add 45.45.1.1/24 dev ipip11

$hexec ep1 ip link add name ipip12 type ipip local 31.31.31.1 remote 31.31.31.254
$hexec ep1 ip link set dev ipip12 up
$hexec ep1 ip addr add 45.45.2.2/24 dev ipip12

#$hexec ep1 ip route add 10.10.10.0/24 dev ipip0
$hexec ep1 ip addr add 56.56.56.1/32 dev lo
$hexec ep1 ip addr add 20.20.20.1/32 dev lo

$hexec ep2 ip link add name ipip21 type ipip local 32.32.32.1 remote 32.32.32.253
$hexec ep2 ip link set dev ipip21 up
$hexec ep2 ip addr add 46.46.1.1/24 dev ipip21

$hexec ep2 ip link add name ipip22 type ipip local 32.32.32.1 remote 32.32.32.254
$hexec ep2 ip link set dev ipip22 up
$hexec ep2 ip addr add 46.46.2.1/24 dev ipip22

#$hexec ep2 ip route add 10.10.10.0/24 dev ipip0
$hexec ep2 ip addr add 57.57.57.1/32 dev lo
$hexec ep2 ip addr add 20.20.20.1/32 dev lo

$hexec ep3 ip link add name ipip31 type ipip local 33.33.33.1 remote 33.33.33.253
$hexec ep3 ip link set dev ipip31 up
$hexec ep3 ip addr add 47.47.1.1/24 dev ipip31
$hexec ep3 ip link add name ipip32 type ipip local 33.33.33.1 remote 33.33.33.254
$hexec ep3 ip link set dev ipip32 up
$hexec ep3 ip addr add 47.47.2.1/24 dev ipip32

#$hexec ep3 ip route add 10.10.10.0/24 dev ipip0
$hexec ep3 ip addr add 58.58.58.1/32 dev lo
$hexec ep3 ip addr add 20.20.20.1/32 dev lo

##Pod networks
#add_route llb1 1.1.1.0/24 11.11.11.254
#add_route llb2 1.1.1.0/24 11.11.11.254

sleep 5
##Create LB rule
create_lb_rule llb1 20.20.20.1 --select=hash --tcp=8080:8080 --endpoints=56.56.56.1:1,57.57.57.1:1,58.58.58.1:1 --mode=dsr --bgp
create_lb_rule llb2 20.20.20.1 --select=hash --tcp=8080:8080 --endpoints=56.56.56.1:1,57.57.57.1:1,58.58.58.1:1 --mode=dsr --bgp

sleep 2
$dexec llb1 loxicmd save --lb
$dexec llb2 loxicmd save --lb
