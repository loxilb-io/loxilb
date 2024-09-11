#!/bin/bash

echo "#########################################"
echo "Removing testbed"
echo "#########################################"

source /vagrant/common.sh

disconnect_docker_hosts user r1
disconnect_docker_hosts user r2
disconnect_docker_hosts r1 sw1
disconnect_docker_hosts r2 sw1
disconnect_docker_hosts sw1 llb1
disconnect_docker_hosts sw1 llb2
disconnect_docker_hosts llb1 sw2
disconnect_docker_hosts llb2 sw2
disconnect_docker_hosts sw2 r3
disconnect_docker_hosts sw2 r4
disconnect_docker_hosts r3 ep1
disconnect_docker_hosts r4 ep1

delete_docker_host user
delete_docker_host llb1
delete_docker_host llb2
delete_docker_host r1
delete_docker_host r2
delete_docker_host r3
delete_docker_host r4
delete_docker_host sw1
delete_docker_host sw2
delete_docker_host ep1

echo "#########################################"
echo "Removed testbed"
echo "#########################################"
