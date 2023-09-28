#!/bin/bash

echo "#########################################"
echo "Removing testbed"
echo "#########################################"

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

source ../common.sh

sudo k0s kubectl delete -f nginx-svc-lb1.yml >> /dev/null 2>&1
sudo k0s kubectl delete -f udp-svc-lb.yml >> /dev/null 2>&1
sudo k0s kubectl delete -f sctp-svc-lb.yml >> /dev/null 2>&1
sudo k0s kubectl delete -f udp-svc-lb2.yml >> /dev/null 2>&1
sudo k0s kubectl delete -f sctp-svc-lb2.yml >> /dev/null 2>&1
sudo k0s kubectl delete -f kube-loxilb.yml >> /dev/null 2>&1

disconnect_docker_hosts user r1
disconnect_docker_hosts r1 llb1
disconnect_docker_hosts r1 llb2
disconnect_docker_hosts llb1 r2
disconnect_docker_hosts llb2 r2
disconnect_docker_hosts r2 ep1
disconnect_docker_hosts r2 ep2
disconnect_docker_hosts r2 ep3

delete_docker_host ka_llb1
delete_docker_host ka_llb2
delete_docker_host llb1
delete_docker_host llb2
delete_docker_host user
delete_docker_host r1
delete_docker_host r2
delete_docker_host ep1
delete_docker_host ep2
delete_docker_host ep3
sudo ip link del esysllb1
sudo ip link del esysllb2

./rmweave.sh

# If k3s setup exists, remove it
if [[ -f "/usr/local/bin/k0s" ]]; then
  sudo k0s stop
  sudo k0s reset
  sudo rm -f /usr/local/bin/k0s
fi

sudo apt-get remove bird2 --yes

echo "#########################################"
echo "Removed testbed"
echo "#########################################"
