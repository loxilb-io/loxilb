#!/bin/bash

echo "#########################################"
echo "Removing testbed"
echo "#########################################"

source ../common.sh

disconnect_docker n1p1 llb1
disconnect_docker n1p2 llb1
disconnect_docker k8n1 llb1
disconnect_docker n2p1 k8n1
disconnect_docker n2p2 k8n1

delete_docker_host loxilb llb1
delete docker_host host n1p1
delete docker_host host n1p2
delete docker_host host n2p1
delete_docker_host host n3p1
delete docker_host host k8n1

echo "#########################################"
echo "Removed testbed"
echo "#########################################"
