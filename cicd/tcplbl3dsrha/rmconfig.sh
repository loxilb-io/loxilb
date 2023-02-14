#!/bin/bash

echo "#########################################"
echo "Removing testbed"
echo "#########################################"

source ../common.sh

disconnect_docker_hosts user r1
disconnect_docker_hosts r1 r2
disconnect_docker_hosts r1 llb1
disconnect_docker_hosts r1 llb2
disconnect_docker_hosts llb1 ep1
disconnect_docker_hosts llb1 ep2
disconnect_docker_hosts llb1 ep3
disconnect_docker_hosts llb2 ep1
disconnect_docker_hosts llb2 ep2
disconnect_docker_hosts llb2 ep3
disconnect_docker_hosts ep1 r2
disconnect_docker_hosts ep2 r2
disconnect_docker_hosts ep3 r2


delete_docker_host llb1
delete_docker_host llb2
delete_docker_host user
delete_docker_host r1
delete_docker_host r2
delete_docker_host ep1
delete_docker_host ep2
delete_docker_host ep3

echo "#########################################"
echo "Removed testbed"
echo "#########################################"
