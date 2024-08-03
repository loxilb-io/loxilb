#!/bin/bash
source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name lgw1
spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type loxilb --dock-name rgw1
spawn_docker_host --dock-type loxilb --dock-name rgw2
spawn_docker_host --dock-type host --dock-name lh1
spawn_docker_host --dock-type host --dock-name rh1
spawn_docker_host --dock-type host --dock-name rh2

$dexec lgw1 bash -c "apt-get update && apt-get install -y iputils-ping curl"
$dexec llb1 bash -c "apt-get update && apt-get install -y iputils-ping curl"
$dexec rgw1 bash -c "apt-get update && apt-get install -y iputils-ping curl"
$dexec rgw2 bash -c "apt-get update && apt-get install -y iputils-ping curl"

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts lh1 lgw1
connect_docker_hosts lgw1 llb1
connect_docker_hosts llb1 rgw1
connect_docker_hosts llb1 rgw2
connect_docker_hosts rh1 rgw1
connect_docker_hosts rh2 rgw2

config_docker_host --host1 lh1 --host2 lgw1 --ptype phy --addr 192.168.10.175/24 --gw 192.168.10.1
config_docker_host --host1 lgw1 --host2 lh1 --ptype phy --addr 192.168.10.1/24
config_docker_host --host1 lgw1 --host2 llb1 --ptype phy --addr 7.7.7.1/24
config_docker_host --host1 llb1 --host2 lgw1 --ptype phy --addr 7.7.7.254/24

#Tunnel 1
#xfrm Config(Left)
$dexec lgw1 ip link add vti100 type vti key 100 remote 7.7.7.254 local 7.7.7.1
$dexec lgw1 ip link set vti100 up
$dexec lgw1 ip addr add 77.77.77.1/24 remote 77.77.77.254/24 dev vti100
$dexec lgw1 sysctl -w "net.ipv4.conf.vti100.disable_policy=1"
$dexec lgw1 sysctl -w "net.ipv4.conf.elgw1lh1.proxy_arp=1"

$dexec lgw1 ip route add 192.168.10.200/32 via 77.77.77.254

#xfrm Config(Right)
$dexec llb1 ip link add vti100 type vti key 100 remote 7.7.7.1 local 7.7.7.254
$dexec llb1 ip link set vti100 up
$dexec llb1 ip addr add 77.77.77.254/24 remote 77.77.77.1/24 dev vti100
$dexec llb1 sysctl -w "net.ipv4.conf.vti100.disable_policy=1"
#$dexec llb1 sysctl -w "net.ipv4.conf.ellb1lgw1.proxy_arp=1"

$dexec llb1 ip addr add 192.168.10.200/32 dev lo
$dexec llb1 ip route add 192.168.10.175/32 via 77.77.77.1 dev vti100
$dexec llb1 loxicmd create lb 192.168.10.200 --tcp=2020:8080 --endpoints=192.168.10.10:1,192.168.10.11:1 --mode=fullnat
$dexec llb1 loxicmd create ep 192.168.10.10 --name=192.168.10.10_tcp_2020 --probetype=none
$dexec llb1 loxicmd create ep 192.168.10.11 --name=192.168.10.11_tcp_2020 --probetype=none

#Route towards Host(lh1)
$dexec llb1 ip route add 192.168.10.175/32 via 77.77.77.1 dev vti100



create_docker_host_vlan --host1 llb1 --host2 rgw1 --id 1000 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 rgw2 --id 1000 --ptype untagged

config_docker_host --host1 rgw1 --host2 llb1 --ptype phy --addr 8.7.7.1/24
config_docker_host --host1 rgw2 --host2 llb1 --ptype phy --addr 8.7.7.2/24

config_docker_host --host1 llb1 --host2 rgw1 --ptype vlan --id 1000 --addr 8.7.7.254/24

#Tunnel-2

#xfrm Config(Right)
$dexec llb1 ip link add vti200 type vti key 200 remote 8.7.7.1 local 8.7.7.254
$dexec llb1 ip link set vti200 up
$dexec llb1 ip addr add 8.7.200.254/24 remote 8.7.200.1/24 dev vti200
$dexec llb1 sysctl -w "net.ipv4.conf.vti200.disable_policy=1"

#Route towards EP(rh1)
$dexec llb1 ip route add 192.168.10.10/32 via 8.7.200.1 dev vti200


#xfrm Config(Left)
$dexec rgw1 ip link add vti200 type vti key 200 remote 8.7.7.254 local 8.7.7.1
$dexec rgw1 ip link set vti200 up
$dexec rgw1 ip addr add 8.7.200.1/24 remote 8.7.200.254/24 dev vti200
$dexec rgw1 sysctl -w "net.ipv4.conf.vti200.disable_policy=1"
$dexec rgw1 sysctl -w "net.ipv4.conf.ergw1rh1.proxy_arp=1"
#Route towards llb1
$dexec rgw1 ip route add 192.168.10.200/32 via 8.7.200.254


#Tunnel-3

#xfrm Config(Right)
$dexec llb1 ip link add vti201 type vti key 201 remote 8.7.7.2 local 8.7.7.254
$dexec llb1 ip link set vti201 up
$dexec llb1 ip addr add 8.7.201.254/24 remote 8.7.201.1/24 dev vti201
$dexec llb1 sysctl -w "net.ipv4.conf.vti201.disable_policy=1"

#Route towards EP(rh2)
$dexec llb1 ip route add 192.168.10.11/32 via 8.7.201.1 dev vti201

$dexec rgw2 ip link add vti201 type vti key 201 remote 8.7.7.254 local 8.7.7.2
$dexec rgw2 ip link set vti201 up
$dexec rgw2 ip addr add 8.7.201.1/24 remote 8.7.201.254/24 dev vti201
$dexec rgw2 sysctl -w "net.ipv4.conf.vti201.disable_policy=1"
$dexec rgw2 sysctl -w "net.ipv4.conf.ergw2rh2.proxy_arp=1"
#Route towards llb1
$dexec rgw2 ip route add 192.168.10.200/32 via 8.7.201.254



config_docker_host --host1 rgw1 --host2 rh1 --ptype phy --addr 192.168.10.2/24
config_docker_host --host1 rh1 --host2 rgw1 --ptype phy --addr 192.168.10.10/24 --gw 192.168.10.2

config_docker_host --host1 rgw2 --host2 rh2 --ptype phy --addr 192.168.10.3/24
config_docker_host --host1 rh2 --host2 rgw2 --ptype phy --addr 192.168.10.11/24 --gw 192.168.10.3

#$dexec lgw1 apt-get update
$dexec lgw1 apt-get install -y iptables strongswan strongswan-swanctl systemctl 
docker cp lgw1_ipsec_config/ipsec.conf lgw1:/etc/
docker cp lgw1_ipsec_config/ipsec.secrets lgw1:/etc/
docker cp lgw1_ipsec_config/charon.conf lgw1:/etc/strongswan.d/
$dexec lgw1 systemctl restart strongswan-starter

#$dexec llb1 apt-get update
$dexec llb1 apt-get install -y strongswan strongswan-swanctl systemctl
docker cp llb1_ipsec_config/ipsec.conf llb1:/etc/
docker cp llb1_ipsec_config/ipsec.secrets llb1:/etc/
docker cp llb1_ipsec_config/charon.conf llb1:/etc/strongswan.d/
$dexec llb1 systemctl restart strongswan-starter

#$dexec rgw1 apt-get update
$dexec rgw1 apt-get install -y iptables strongswan strongswan-swanctl systemctl 
docker cp rgw1_ipsec_config/ipsec.conf rgw1:/etc/
docker cp rgw1_ipsec_config/ipsec.secrets rgw1:/etc/
docker cp rgw1_ipsec_config/charon.conf rgw1:/etc/strongswan.d/
$dexec rgw1 systemctl restart strongswan-starter

#$dexec rgw2 apt-get update
$dexec rgw2 apt-get install -y iptables strongswan strongswan-swanctl systemctl 
docker cp rgw2_ipsec_config/ipsec.conf rgw2:/etc/
docker cp rgw2_ipsec_config/ipsec.secrets rgw2:/etc/
docker cp rgw2_ipsec_config/charon.conf rgw2:/etc/strongswan.d/
$dexec rgw2 systemctl restart strongswan-starter

