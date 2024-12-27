#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

NUM_HOSTS=30

spawn_docker_host --dock-type loxilb --dock-name llb1

# Loop to create and configure 100 hosts
for i in $(seq 1 $NUM_HOSTS); do
  host="l3h$i"
  ip="$((i)).$((i)).$((i)).1"
  gateway="$((i)).$((i)).$((i)).254"

  # Spawn a host
  spawn_docker_host --dock-type host --dock-name "$host"
  
  # Connect host to llb1
  connect_docker_hosts "$host" llb1

  # Configure the host
  config_docker_host --host1 "$host" --host2 llb1 --ptype phy --addr "$ip"/24 --gw "$gateway"
  config_docker_host --host1 llb1 --host2 "$host" --ptype phy --addr "$gateway"/24
done

spawn_docker_host --dock-type host --dock-name l3ep1
spawn_docker_host --dock-type host --dock-name l3ep2
spawn_docker_host --dock-type host --dock-name l3ep3

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1

config_docker_host --host1 l3ep1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 l3ep2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 l3ep3 --host2 llb1 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254
config_docker_host --host1 llb1 --host2 l3ep1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 l3ep2 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 llb1 --host2 l3ep3 --ptype phy --addr 33.33.33.254/24

# Create LB rule
$dexec llb1 loxicmd create lb 150.150.150.1 --tcp=2020:8001-8100 --endpoints=31.31.31.1:1,32.32.32.1:1 --select=persist --mode=onearm --inatimeout=40
