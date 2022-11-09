#!/bin/bash

source ../common.sh

disconnect_docker_hosts l2h1 llb1
disconnect_docker_hosts l2ep1 llb1
disconnect_docker_hosts l2ep2 llb1
disconnect_docker_hosts l2ep3 llb1

delete_docker_host llb1
delete_docker_host l2h1
delete_docker_host l2ep1
delete_docker_host l2ep2
delete_docker_host l2ep3

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
