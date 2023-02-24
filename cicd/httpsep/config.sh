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

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts l3h1 llb1
connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1

sleep 5

#L3 config
config_docker_host --host1 l3h1 --host2 llb1 --ptype phy --addr 10.10.10.1/24 --gw 10.10.10.254
config_docker_host --host1 l3ep1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 l3ep2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 l3ep3 --host2 llb1 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254
config_docker_host --host1 llb1 --host2 l3h1 --ptype phy --addr 10.10.10.254/24
config_docker_host --host1 llb1 --host2 l3ep1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 l3ep2 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 llb1 --host2 l3ep3 --ptype phy --addr 33.33.33.254/24

sleep 5

#Generate Certificates for client
./minica -ip-addresses 10.10.10.1

#Generate Certificates for loxilb
./minica -ip-addresses 31.31.31.254,32.32.32.254,33.33.33.254

#Generate Certificates for Endpoints
./minica -ip-addresses 31.31.31.1,20.20.20.1
./minica -ip-addresses 32.32.32.1,20.20.20.1
./minica -ip-addresses 33.33.33.1,20.20.20.1

docker cp minica.pem llb1:/opt/loxilb/cert/rootCA.crt
docker cp 31.31.31.254/cert.pem llb1:/opt/loxilb/cert/server.crt
docker cp 31.31.31.254/key.pem llb1:/opt/loxilb/cert/server.key

$dexec llb1 pkill loxilb
$dexec llb1 ip link del llb0
docker exec -dt llb1 /root/loxilb-io/loxilb/loxilb

sleep 5

$dexec llb1 loxicmd create endpoint 31.31.31.1 --probetype=https --probeport=8080 --probereq="health" --proberesp="OK" --period=60 --retries=2
$dexec llb1 loxicmd create endpoint 32.32.32.1 --probetype=https --probeport=8080 --probereq="health" --proberesp="OK" --period=60 --retries=2
$dexec llb1 loxicmd create endpoint 33.33.33.1 --probetype=https --probeport=8080 --probereq="health" --proberesp="OK" --period=60 --retries=2

$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --monitor

sleep 10
