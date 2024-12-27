#!/bin/bash

export OSE_LOXILB_SERVERS=${OSE_LOXILB_SERVERS:-1}

source ../common.sh

disconnect_docker_hosts l3h1 llb1
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
    disconnect_docker_hosts l3ep$i llb1
done

delete_docker_host llb1
delete_docker_host l3h1
for i in $(seq 1 $OSE_LOXILB_SERVERS)
do
    delete_docker_host l3ep$i
done

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
