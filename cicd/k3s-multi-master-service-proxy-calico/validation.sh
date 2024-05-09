#!/bin/bash
source ../common.sh
echo k3s-flannel-cluster-l2

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 5
extIP="192.168.80.200"
echo $extIP
echo $extIP > extIP

echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh master1 -c 'sudo kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nCluster Info"
echo "******************************************************************************"
echo "******************************************************************************"
echo -e "\nPods"
echo "******************************************************************************"
vagrant ssh master1 -c 'sudo kubectl get pods -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nNodes"
echo "******************************************************************************"
vagrant ssh master1 -c 'sudo kubectl get nodes' 2> /dev/null

vagrant ssh host -c 'sudo /vagrant/host_validation.sh' 2> /dev/null
sudo rm extIP
