#!/bin/bash

source ../common.sh

disconnect_docker_hosts lh1 llb1
disconnect_docker_hosts lh2 llb1
delete_docker_host llb1
delete_docker_host lh1
delete_docker_host lh2

disconnect_docker_hosts rh1 llb2
disconnect_docker_hosts rh2 llb2
delete_docker_host llb2
delete_docker_host rh1
delete_docker_host rh2

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
