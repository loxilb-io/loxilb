#!/bin/bash
source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

if [[ "$1" == "add" ]]; then
spawn_docker_host --dock-type loxilb --dock-name llb1 --with-bgp yes --bgp-config $(pwd)/llb1_gobgp_config --with-ka yes --ka-config $(pwd)/keepalived_config
#spawn_docker_host loxilb llb1

else

delete_docker_host llb1
fi
