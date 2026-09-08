#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

# --vip-adv-dev pins the advertisement to the VLAN 11 segment. Automatic
# resolution cannot get this topology right: llb1 and llb2 have no route to
# 20.20.20.1 of their own, so the only main table route covering the VIP is the
# docker default route out of eth0, and eth0 is a loxilb port like any other.
# That is the multi-homed case the option exists for.
#
# Set VIP_ADV_DEV empty to run the same topology on automatic resolution, and
# pass the same value to validation.sh so it knows to drop the GARP assertion.
advdev="${VIP_ADV_DEV-vlan11}"
adv_args=""
if [[ -n "$advdev" ]]; then
    adv_args="--vip-adv-dev $advdev"
fi

spawn_docker_host --dock-type loxilb --dock-name llb1 --with-ka out --ka-config $(pwd)/keepalived_config --extra-args "$adv_args"
spawn_docker_host --dock-type loxilb --dock-name llb2 --with-ka out --ka-config $(pwd)/keepalived_config --extra-args "$adv_args"
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

##IPv6 on the same segment
# spawn_docker_host turns IPv6 off in every container, and clearing the global
# knob does not reach vlan11, which already exists by now, so clear that one too.
for h in r1 llb1 llb2; do
  $hexec $h sysctl -w net.ipv6.conf.all.disable_ipv6=0 >/dev/null 2>&1
  $hexec $h sysctl -w net.ipv6.conf.default.disable_ipv6=0 >/dev/null 2>&1
  $hexec $h sysctl -w net.ipv6.conf.vlan11.disable_ipv6=0 >/dev/null 2>&1
done

$hexec r1   ip -6 addr add 2001:db8:11::254/64 dev vlan11 nodad
$hexec llb1 ip -6 addr add 2001:db8:11::1/64 dev vlan11 nodad
$hexec llb2 ip -6 addr add 2001:db8:11::2/64 dev vlan11 nodad

$hexec r1 ip route add 20.20.20.1/32 via 11.11.11.11
add_route llb1 1.1.1.0/24 11.11.11.254
add_route llb2 1.1.1.0/24 11.11.11.254

sleep 1

##Create LB rule
create_lb_rule llb1 20.20.20.1 --tcp=2020:8080 --endpoints=11.11.11.3:1,11.11.11.4:1,11.11.11.5:1 --mode=fullnat
create_lb_rule llb2 20.20.20.1 --tcp=2020:8080 --endpoints=11.11.11.3:1,11.11.11.4:1,11.11.11.5:1 --mode=fullnat

##IPv6 VIP, L2 adjacent to the VLAN 11 subnet
# Traffic to this one is not exercised - the endpoints have no IPv6 of their
# own. It is here for where the /128 ends up: the master must carry it and the
# backup must not, which is what the add and delete paths have to agree on.
create_lb_rule llb1 2001:db8:11::100 --tcp=2020:8080 --endpoints=2001:db8:11::3:1 --mode=fullnat
create_lb_rule llb2 2001:db8:11::100 --tcp=2020:8080 --endpoints=2001:db8:11::3:1 --mode=fullnat

# keepalive will take few seconds to be UP and running with valid states
sleep 10
