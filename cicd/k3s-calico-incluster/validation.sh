#!/bin/bash
source ../common.sh
echo k3s-calico-incluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 5
extIP="123.123.123.1"
echo $extIP

echo "Service Info"
vagrant ssh master1 -c 'sudo kubectl get svc'

print_debug_info() {
  echo "cluster-info"
  vagrant ssh master1 -c 'sudo kubectl get pods -A'
  vagrant ssh master1 -c 'sudo kubectl get svc'
  vagrant ssh master1 -c 'sudo kubectl get nodes'
}

out=$(curl -s --connect-timeout 10 http://$extIP:55002) 
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "k3s-calico-incluster (kube-loxilb) tcp [OK]"
else
  echo "k3s-calico-incluster (kube-loxilb) tcp [FAILED]"
  print_debug_info
  exit 1
fi

out=$(timeout 10 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "k3s-calico-incluster (kube-loxilb) udp [OK]"
else
  echo "k3s-calico-incluster (kube-loxilb) udp [FAILED]"
  print_debug_info
  exit 1
fi

exit
