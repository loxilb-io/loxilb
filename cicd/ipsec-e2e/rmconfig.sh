#!/bin/bash

source ../common.sh

disconnect_docker_hosts lh1 lgw1
disconnect_docker_hosts lgw1 llb1
disconnect_docker_hosts llb1 rgw1
disconnect_docker_hosts llb1 rgw2
disconnect_docker_hosts rgw1 rh1
disconnect_docker_hosts rgw2 rh2

delete_docker_host llb1
delete_docker_host lgw1
delete_docker_host rgw1
delete_docker_host rgw2
delete_docker_host lh1
delete_docker_host rh1
delete_docker_host rh2

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
