#!/bin/bash
source ../common.sh
echo sharding-test

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

# config.sh waits for the workloads and writes the external addresses that
# kube-loxilb actually assigned. Refuse to run against guessed addresses.
for f in extIP extIP1 extIP2; do
  if [[ ! -s $f ]]; then
    echo "$f is missing or empty; config.sh did not complete" >&2
    exit 1
  fi
done
extIP=$(cat extIP)
extIP1=$(cat extIP1)
extIP2=$(cat extIP2)
echo "TCP $extIP UDP $extIP1 SCTP $extIP2"

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

# The verdict is host_validation.sh's exit code. It used to be lost behind the
# rm that followed, so this script always returned 0.
vagrant ssh host -c 'sudo /vagrant/host_validation.sh'
rc=$?
rm -f extIP
exit $rc
