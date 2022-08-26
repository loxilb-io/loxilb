#!/bin/bash

source ../common.sh

disconnect_docker_hosts ue1 llb1
disconnect_docker_hosts ue2 llb1
delete_docker_host llb1
delete_docker_host ue1
delete_docker_host ue2

disconnect_docker_hosts l3e1 llb2
disconnect_docker_hosts l3e2 llb2
disconnect_docker_hosts l3e3 llb2
delete_docker_host llb2
delete_docker_host l3e1
delete_docker_host l3e2
delete_docker_host l3e3

echo "#########################################"
echo "Deleted stale testbed"
echo "#########################################"

sleep 5

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host loxilb llb1
spawn_docker_host loxilb llb2
spawn_docker_host host ue1
spawn_docker_host host ue2
spawn_docker_host host l3e1
spawn_docker_host host l3e2
spawn_docker_host host l3e3

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts ue1 llb1
connect_docker_hosts ue2 llb1
connect_docker_hosts llb1 llb2

config_docker_host --host1 ue1 --host2 llb1 --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 llb1 --host2 ue1 32.32.32.254/24
config_docker_host --host1 ue2 --host2 llb1 31.31.31.1/24 31.31.31.254
config_docker_host --host1 llb1 --host2 ue2 31.31.31.254/24
config_docker_host --host1 llb1 --host2 llb2 10.10.10.59/24
config_docker_host --host1 llb2 --host2 llb1 10.10.10.56/24

connect_docker_hosts l3e1 llb2
connect_docker_hosts l3e2 llb2
connect_docker_hosts l3e3 llb2

config_docker_host --host1 l3e1 --host2 llb2 --addr 25.25.25.1/24 --gw 25.25.25.254
config_docker_host --host1 llb2 --host2 l3e1 --addr 25.25.25.254/24
config_docker_host --host1 l3e2 --host2 llb2 --addr 26.26.26.1/24 --gw 26.26.26.254
config_docker_host --host1 llb2 --host2 l3e2 --addr 26.26.26.254/24
config_docker_host --host1 l3e3 --host2 llb2 --addr 27.27.27.1/24 --gw 27.27.27.254
config_docker_host --host1 llb2 --host2 l3e3 --addr 27.27.27.254/24

$hexec llb1 ip route add 25.25.25.0/24 via 10.10.10.56 dev enpllb1llb2
$hexec llb1 ip route add 26.26.26.0/24 via 10.10.10.56 dev enpllb1llb2
$hexec llb1 ip route add 27.27.27.0/24 via 10.10.10.56 dev enpllb1llb2

$hexec llb2 ip route add 31.31.31.0/24 via 10.10.10.59 dev enpllb2llb1
$hexec llb2 ip route add 32.32.32.0/24 via 10.10.10.59 dev enpllb2llb1

