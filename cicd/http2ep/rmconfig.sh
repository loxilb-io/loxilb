#!/bin/bash

source ../common.sh

disconnect_docker_hosts l3h1 llb1
disconnect_docker_hosts l3ep1 llb1
disconnect_docker_hosts l3ep2 llb1
disconnect_docker_hosts l3ep3 llb1

delete_docker_host llb1
delete_docker_host l3h1
delete_docker_host l3ep1
delete_docker_host l3ep2
delete_docker_host l3ep3

rm -rf 10.10.10.1 31.31.31.* 32.32.32.1 33.33.33.1 minica.pem minica-key.pem

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
