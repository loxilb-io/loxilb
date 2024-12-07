#!/bin/bash
source ../common.sh
echo dual-stack-test

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 45
extIP="3ffe:cafe::1"
echo $extIP
echo $extIP > extIP

echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh master -c 'sudo kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nCluster Info"
echo "******************************************************************************"
echo "******************************************************************************"
echo -e "\nPods"
echo "******************************************************************************"
vagrant ssh master -c 'sudo kubectl get pods -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nNodes"
echo "******************************************************************************"
vagrant ssh master -c 'sudo kubectl get nodes' 2> /dev/null

vagrant ssh host -c 'sudo /vagrant/host_validation.sh' 2> /dev/null
sudo rm extIP
