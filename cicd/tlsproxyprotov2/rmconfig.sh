#!/bin/bash

source ../common.sh

# Best-effort stop of the backend nginx instances.
for ep in l3ep1 l3ep2 l3ep3; do
  $dexec "$ep" nginx -s stop 2>/dev/null || true
done

disconnect_docker_hosts l3h1 llb1
disconnect_docker_hosts l3ep1 llb1
disconnect_docker_hosts l3ep2 llb1
disconnect_docker_hosts l3ep3 llb1

delete_docker_host llb1
delete_docker_host l3h1
delete_docker_host l3ep1
delete_docker_host l3ep2
delete_docker_host l3ep3

rm -rf 10.10.10.254/ minica.pem minica-key.pem nginx.l3ep*.conf apt.l3ep*.log

echo "#########################################"
echo "Deleted testbed"
echo "#########################################"
