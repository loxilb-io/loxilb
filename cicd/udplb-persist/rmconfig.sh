#!/bin/bash

source ../common.sh

sudo pkill udp_server 2>&1 > /dev/null

disconnect_docker_hosts l3h1 llb1
disconnect_docker_hosts l3h2 llb1
disconnect_docker_hosts l3h3 llb1
disconnect_docker_hosts l3ep1 llb1
disconnect_docker_hosts l3ep2 llb1
disconnect_docker_hosts l3ep3 llb1

delete_docker_host llb1
delete_docker_host l3h1
delete_docker_host l3h2
delete_docker_host l3h3
delete_docker_host l3ep1
delete_docker_host l3ep2
delete_docker_host l3ep3

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
