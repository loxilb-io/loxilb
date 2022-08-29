#!/bin/bash

echo "#########################################"
echo "Removing testbed"
echo "#########################################"

source ../common.sh

disconnect_docker_hosts n1p1 llb1
disconnect_docker_hosts n1p2 llb1
disconnect_docker_hosts k8n1 llb1
disconnect_docker_hosts n2p1 k8n1
disconnect_docker_hosts n3p1 k8n1

delete_docker_host llb1
delete_docker_host n1p1
delete_docker_host n1p2
delete_docker_host n2p1
delete_docker_host n3p1
delete_docker_host k8n1

echo "#########################################"
echo "Removed testbed"
echo "#########################################"
