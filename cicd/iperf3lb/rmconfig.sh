#!/bin/bash

source ../common.sh

sudo pkill -9 iperf3 2>&1 > /dev/null

disconnect_docker_hosts l3ep1 llb1
disconnect_docker_hosts l3ep2 llb1
disconnect_docker_hosts l3ep3 llb1

NUM_HOSTS=30
for i in $(seq 1 $NUM_HOSTS); do
  host="l3h$i"
  disconnect_docker_hosts $host llb1
  delete_docker_host $host
done

delete_docker_host llb1
delete_docker_host l3ep1
delete_docker_host l3ep2
delete_docker_host l3ep3

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
