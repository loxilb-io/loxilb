#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type host --dock-name l3h1
spawn_docker_host --dock-type host --dock-name l3ep1
spawn_docker_host --dock-type host --dock-name l3ep2
spawn_docker_host --dock-type host --dock-name l3ep3
spawn_docker_host --dock-type host --dock-name l3ep4

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts l3h1 llb1
connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1
connect_docker_hosts l3ep4 llb1

sleep 5

#configure pods
config_docker_host --host1 l3h1 --host2 llb1 --ptype phy --addr 10.10.10.1/24 --gw 10.10.10.254
config_docker_host --host1 l3ep1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 l3ep2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 l3ep3 --host2 llb1 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254
config_docker_host --host1 l3ep4 --host2 llb1 --ptype phy --addr 34.34.34.1/24 --gw 34.34.34.254
config_docker_host --host1 llb1 --host2 l3h1 --ptype phy --addr 10.10.10.254/24
config_docker_host --host1 llb1 --host2 l3ep1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 l3ep2 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 llb1 --host2 l3ep3 --ptype phy --addr 33.33.33.254/24
config_docker_host --host1 llb1 --host2 l3ep4 --ptype phy --addr 34.34.34.254/24

for i in {1..4}
do
for j in {1..8}
do
  $hexec l3ep$i ip addr add 35.$i.$j.1/24 dev el3ep${i}llb1
  $hexec llb1 ip addr add 35.$i.$j.254/24 dev ellb1l3ep${i}
done
done

sleep 5

#configure LB

create_lb_rule llb1 20.20.20.1 --tcp=2020:8080 --endpoints=35.1.1.1:1,35.1.2.1:1,35.1.3.1:1,35.1.4.1:1,35.1.5.1:1,35.1.6.1:1,35.1.7.1:1,35.1.8.1:1,35.2.1.1:1,35.2.2.1:1,35.2.3.1:1,35.2.4.1:1,35.2.5.1:1,35.2.6.1:1,35.2.7.1:1,35.2.8.1:1,35.3.1.1:1,35.3.2.1:1,35.3.3.1:1,35.3.4.1:1,35.3.5.1:1,35.3.6.1:1,35.3.7.1:1,35.3.8.1:1,35.4.1.1:1,35.4.2.1:1,35.4.3.1:1,35.4.4.1:1,35.4.5.1:1,35.4.6.1:1,35.4.7.1:1,35.4.8.1:1
