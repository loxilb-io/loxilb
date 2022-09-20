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
echo "Deleted testbed"
echo "#########################################"
