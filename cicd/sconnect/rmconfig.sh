#!/bin/bash

source ../common.sh

disconnect_docker_hosts l3h1 llb1
disconnect_docker_hosts l3h2 llb1
disconnect_docker_hosts l3h3 llb1
disconnect_docker_hosts l2h1 llb1
disconnect_docker_hosts l2h2 llb1
disconnect_docker_hosts l2h3 llb1
disconnect_docker_hosts l2h4 llb1
disconnect_docker_hosts l2h5 llb1
disconnect_docker_hosts l2h6 llb1
disconnect_docker_hosts l2vxh1 llb1
disconnect_docker_hosts l2vxh2 llb1
disconnect_docker_hosts l2vxh3 llb1
disconnect_docker_hosts l2vxh4 llb1
disconnect_docker_hosts l2vxh5 llb1
disconnect_docker_hosts l2vxh6 llb1
disconnect_docker_hosts l2vxh7 llb1
disconnect_docker_hosts l2vxh8 llb1
disconnect_docker_hosts l2vxh9 llb1
disconnect_docker_hosts l3vxh1 llb1
disconnect_docker_hosts l3vxh2 llb1
disconnect_docker_hosts l3vxh3 llb1

delete_docker_host llb1
delete_docker_host l3h1
delete_docker_host l3h2
delete_docker_host l3h3
delete_docker_host l2h1
delete_docker_host l2h2
delete_docker_host l2h3
delete_docker_host l2h4
delete_docker_host l2h5
delete_docker_host l2h6
delete_docker_host l2vxh1
delete_docker_host l2vxh2
delete_docker_host l2vxh3
delete_docker_host l2vxh4
delete_docker_host l2vxh5
delete_docker_host l2vxh6
delete_docker_host l2vxh7
delete_docker_host l2vxh8
delete_docker_host l2vxh9
delete_docker_host l3vxh1
delete_docker_host l3vxh2
delete_docker_host l3vxh3

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
