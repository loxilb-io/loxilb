#!/bin/bash

source /vagrant/common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1 --with-ka in
spawn_docker_host --dock-type loxilb --dock-name llb2 --with-ka in
spawn_docker_host --dock-type seahost --dock-name ep1
spawn_docker_host --dock-type host --dock-name r1
spawn_docker_host --dock-type host --dock-name r2
spawn_docker_host --dock-type host --dock-name r3
spawn_docker_host --dock-type host --dock-name r4
spawn_docker_host --dock-type host --dock-name sw1
spawn_docker_host --dock-type host --dock-name sw2
spawn_docker_host --dock-type seahost --dock-name user

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts user r1
connect_docker_hosts user r2
connect_docker_hosts r1 sw1
connect_docker_hosts r2 sw1
connect_docker_hosts sw1 llb1
connect_docker_hosts sw1 llb2
connect_docker_hosts llb1 sw2
connect_docker_hosts llb2 sw2
connect_docker_hosts sw2 r3
connect_docker_hosts sw2 r4
connect_docker_hosts r3 ep1
connect_docker_hosts r4 ep1

create_docker_host_cnbridge --host1 sw1 --host2 llb1
create_docker_host_cnbridge --host1 sw1 --host2 llb2
create_docker_host_cnbridge --host1 sw1 --host2 r1
create_docker_host_cnbridge --host1 sw1 --host2 r2

create_docker_host_cnbridge --host1 sw2 --host2 llb1
create_docker_host_cnbridge --host1 sw2 --host2 llb2
create_docker_host_cnbridge --host1 sw2 --host2 r3
create_docker_host_cnbridge --host1 sw2 --host2 r4

#node1 config
config_docker_host --host1 user --host2 r1 --ptype phy --addr 1.1.1.1/24 --gw 1.1.1.254
config_docker_host --host1 user --host2 r2 --ptype phy --addr 2.2.2.1/24
config_docker_host --host1 r1 --host2 user --ptype phy --addr 1.1.1.254/24
config_docker_host --host1 r2 --host2 user --ptype phy --addr 2.2.2.254/24

create_docker_host_vlan --host1 llb1 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 r1 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 sw1 --id 11 --ptype untagged
config_docker_host --host1 r1 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.253/24 --gw 11.11.11.11
config_docker_host --host1 r2 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.254/24 --gw 11.11.11.11
config_docker_host --host1 llb1 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.1/24
config_docker_host --host1 llb2 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.2/24

create_docker_host_vlan --host1 llb1 --host2 sw2 --id 10 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 sw2 --id 10 --ptype untagged
create_docker_host_vlan --host1 r3 --host2 sw2 --id 10 --ptype untagged
create_docker_host_vlan --host1 r4 --host2 sw2 --id 10 --ptype untagged

config_docker_host --host1 r3 --host2 sw2 --ptype vlan --id 10 --addr 10.10.10.253/24 --gw 10.10.10.10
config_docker_host --host1 r4 --host2 sw2 --ptype vlan --id 10 --addr 10.10.10.254/24 --gw 10.10.10.10
config_docker_host --host1 llb1 --host2 sw2 --ptype vlan --id 10 --addr 10.10.10.1/24
config_docker_host --host1 llb2 --host2 sw2 --ptype vlan --id 10 --addr 10.10.10.2/24

config_docker_host --host1 ep1 --host2 r3 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 ep1 --host2 r4 --ptype phy --addr 32.32.32.1/24
config_docker_host --host1 r3 --host2 ep1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 r4 --host2 ep1 --ptype phy --addr 32.32.32.254/24

$hexec user ip route change default via 1.1.1.254
$hexec ep1 ip route change default via 31.31.31.254

# Backup paths in user
$hexec user ip route add 21.21.21.1/32 via 2.2.2.254
$hexec user ip route add 134.134.134.1/32 via 2.2.2.254

$hexec ep1 ip route add 134.134.134.1/32 via 32.32.32.254
$hexec ep1 ip route add 135.135.135.1/32 via 31.31.31.254

$hexec llb1 ip route add 1.1.1.0/24 via 11.11.11.253
$hexec llb1 ip route add 2.2.2.0/24 via 11.11.11.254
$hexec llb2 ip route add 1.1.1.0/24 via 11.11.11.253
$hexec llb2 ip route add 2.2.2.0/24 via 11.11.11.254

$hexec llb1 ip route add 31.31.31.0/24 via 10.10.10.253
$hexec llb1 ip route add 32.32.32.0/24 via 10.10.10.254
$hexec llb2 ip route add 31.31.31.0/24 via 10.10.10.253
$hexec llb2 ip route add 32.32.32.0/24 via 10.10.10.254

sleep 20
##Create LB rule user->ep1
create_lb_rule llb1 20.20.20.1 --name=sctpmh1 --secips=21.21.21.1,22.22.22.1 --sctp=2020:8080 --endpoints=31.31.31.1:1 --mode=fullnat
create_lb_rule llb2 20.20.20.1 --name=sctpmh1 --secips=21.21.21.1,22.22.22.1 --sctp=2020:8080 --endpoints=31.31.31.1:1 --mode=fullnat

##Create LB rule ep1->user
create_lb_rule llb1 133.133.133.1 --name=sctpmh2 --secips=134.134.134.1,135.135.135.1 --sctp=2020:8080 --endpoints=1.1.1.1:1 --mode=fullnat
create_lb_rule llb2 133.133.133.1 --name=sctpmh2 --secips=134.134.134.1,135.135.135.1 --sctp=2020:8080 --endpoints=1.1.1.1:1 --mode=fullnat

$dexec llb1 loxicmd create ep 1.1.1.1 --name=1.1.1.1_sctp_8080 --probetype=ping
$dexec llb1 loxicmd create ep 31.31.31.1 --name=31.31.31.1_sctp_8080 --probetype=ping
$dexec llb2 loxicmd create ep 1.1.1.1 --name=1.1.1.1_sctp_8080 --probetype=ping
$dexec llb2 loxicmd create ep 31.31.31.1 --name=31.31.31.1_sctp_8080 --probetype=ping


create_lb_rule llb1 11.11.11.11 --tcp=80:8080 --endpoints=31.31.31.1:1
create_lb_rule llb2 11.11.11.11 --tcp=80:8080 --endpoints=31.31.31.1:1
create_lb_rule llb1 10.10.10.10 --tcp=80:8080 --endpoints=31.31.31.1:1
create_lb_rule llb2 10.10.10.10 --tcp=80:8080 --endpoints=31.31.31.1:1

$dexec llb1 loxicmd save --all
$dexec llb2 loxicmd save --all

$hexec user ifconfig eth0 0
$hexec ep1 ifconfig eth0 0
