#!/bin/bash

source ../common.sh

disconnect_docker_hosts l3h1 llb1
disconnect_docker_hosts l3h1 sw1
disconnect_docker_hosts l3ep1 llb1
disconnect_docker_hosts l3ep2 llb1
disconnect_docker_hosts l3ep3 llb1
disconnect_docker_hosts l3ep1 sw1
disconnect_docker_hosts l3ep2 sw1
disconnect_docker_hosts l3ep3 sw1

delete_docker_host sw1
delete_docker_host llb1
delete_docker_host l3h1
delete_docker_host l3ep1
delete_docker_host l3ep2
delete_docker_host l3ep3

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
