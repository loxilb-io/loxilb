#!/bin/bash

source ../common.sh

disconnect_docker_hosts llb1 br1
disconnect_docker_hosts c1 br1
disconnect_docker_hosts ep1 br1
disconnect_docker_hosts ep2 br1

delete_docker_host llb1
delete_docker_host c1
delete_docker_host ep1
delete_docker_host ep2
delete_docker_host br1

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
