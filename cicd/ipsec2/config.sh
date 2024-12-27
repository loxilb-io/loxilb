#!/bin/bash
source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type loxilb --dock-name llb2
spawn_docker_host --dock-type host --dock-name lh1
spawn_docker_host --dock-type host --dock-name lh2
spawn_docker_host --dock-type host --dock-name rh1
spawn_docker_host --dock-type host --dock-name rh2

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

#Left config
connect_docker_hosts lh1 llb1
connect_docker_hosts lh2 llb1
connect_docker_hosts llb1 llb2

config_docker_host --host1 lh1 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 llb1 --host2 lh1 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 lh2 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 llb1 --host2 lh2 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 llb2 --ptype phy --addr 7.7.7.1/24
config_docker_host --host1 llb2 --host2 llb1 --ptype phy --addr 7.7.7.2/24

#Right Config
connect_docker_hosts rh1 llb2
connect_docker_hosts rh2 llb2

config_docker_host --host1 rh1 --host2 llb2 --ptype phy --addr 25.25.25.1/24 --gw 25.25.25.254
config_docker_host --host1 llb2 --host2 rh1 --ptype phy --addr 25.25.25.254/24
config_docker_host --host1 rh2 --host2 llb2 --ptype phy --addr 26.26.26.1/24 --gw 26.26.26.254
config_docker_host --host1 llb2 --host2 rh2 --ptype phy --addr 26.26.26.254/24

#xfrm Config(Left)
$dexec llb1 ip link add vti100 type vti key 100 remote 7.7.7.2 local 7.7.7.1
$dexec llb1 ip link set vti100 up
$dexec llb1 ip addr add 77.77.77.2/24 remote 77.77.77.1/24 dev vti100
$dexec llb1 sysctl -w "net.ipv4.conf.vti100.disable_policy=1"

$dexec llb1 ip route add 25.25.25.0/24 via 77.77.77.1 dev vti100
$dexec llb1 ip route add 26.26.26.0/24 via 77.77.77.1 dev vti100
$dexec llb1 ip route add 20.20.20.1 via 77.77.77.1 dev vti100

#xfrm Config(Right)
$dexec llb2 ip link add vti100 type vti key 100 remote 7.7.7.1 local 7.7.7.2
$dexec llb2 ip link set vti100 up
$dexec llb2 ip addr add 77.77.77.1/24 remote 77.77.77.2/24 dev vti100
$dexec llb2 sysctl -w "net.ipv4.conf.vti100.disable_policy=1"

$dexec llb2 ip route add 31.31.31.0/24 via 77.77.77.2 dev vti100
$dexec llb2 ip route add 32.32.32.0/24 via 77.77.77.2 dev vti100
$dexec llb2 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=25.25.25.1:1,26.26.26.1:1

$dexec llb1 apt-get update
$dexec llb1 apt-get install -y strongswan strongswan-swanctl systemctl
docker cp llb1_ipsec_config/ipsec.conf llb1:/etc/
docker cp llb1_ipsec_config/ipsec.secrets llb1:/etc/
docker cp llb1_ipsec_config/charon.conf llb1:/etc/strongswan.d/
$dexec llb1 systemctl restart strongswan-starter

$dexec llb2 apt-get update
$dexec llb2 apt-get install -y strongswan strongswan-swanctl systemctl
docker cp llb2_ipsec_config/ipsec.conf llb2:/etc/
docker cp llb2_ipsec_config/ipsec.secrets llb2:/etc/
docker cp llb2_ipsec_config/charon.conf llb2:/etc/strongswan.d/
$dexec llb2 systemctl restart strongswan-starter
