#!/bin/bash

echo "#########################################"
echo "Removing testbed"
echo "#########################################"

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

source ../common.sh

sudo kubectl $KUBECONFIG delete -f nginx-svc-lb.yml >> /dev/null 2>&1
sudo kubectl $KUBECONFIG delete -f nginx.yml >> /dev/null 2>&1
sudo kubectl $KUBECONFIG delete -f nginx-svc-lb1.yml >> /dev/null 2>&1
sudo kubectl $KUBECONFIG delete -f sctp-svc-lb.yml >> /dev/null 2>&1
sudo kubectl $KUBECONFIG delete -f udp-svc-lb.yml >> /dev/null 2>&1
sudo kubectl $KUBECONFIG delete -f kube-loxilb.yml >> /dev/null 2>&1
#sudo kubectl $KUBECONFIG delete -f https://raw.githubusercontent.com/projectcalico/calico/v3.32.2/manifests/custom-resources.yaml >> /dev/null 2>&1
#sudo kubectl $KUBECONFIG delete -f https://raw.githubusercontent.com/projectcalico/calico/v3.32.2/manifests/tigera-operator.yaml >> /dev/null 2>&1
#sudo kubectl $KUBECONFIG delete -f https://raw.githubusercontent.com/projectcalico/calico/v3.32.2/manifests/operator-crds.yaml >> /dev/null 2>&1

disconnect_docker_hosts user r1
disconnect_docker_hosts r1 llb1
disconnect_docker_hosts llb1 r2
disconnect_docker_hosts r2 ep1
disconnect_docker_hosts r2 ep2
disconnect_docker_hosts r2 ep3

delete_docker_host llb1
delete_docker_host user
delete_docker_host r1
delete_docker_host r2
delete_docker_host ep1
delete_docker_host ep2
delete_docker_host ep3
sudo ip link del esysllb1 2>/dev/null
sudo ip link del esysllb2 2>/dev/null

# If k3s setup exists, remove it
if [[ -f "/usr/local/bin/k3s-uninstall.sh" ]]; then
  /usr/local/bin/k3s-uninstall.sh
fi

echo "#########################################"
echo "Removed testbed"
echo "#########################################"
