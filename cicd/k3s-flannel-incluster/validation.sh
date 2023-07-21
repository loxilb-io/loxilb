#!/bin/bash
source ../common.sh
echo k3s-flannel-cluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 45
extIP="123.123.123.1"
echo $extIP

echo "Service Info"
vagrant ssh master -c 'sudo kubectl get svc'
echo "LB Info"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get lb -o wide'
echo "EP Info"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get ep -o wide'

print_debug_info() {
  echo "llb1 route-info"
  vagrant ssh loxilb -c 'ip route'
  vagrant ssh master -c 'sudo kubectl get pods -A'
  vagrant ssh master -c 'sudo kubectl get svc'
  vagrant ssh master -c 'sudo kubectl get nodes'
}

out=$(curl -s --connect-timeout 10 http://$extIP:55002) 
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) tcp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) tcp [FAILED]"
  print_debug_info
  exit 1
fi

out=$(timeout 10 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) udp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) udp [FAILED]"
  print_debug_info
  exit 1
fi

out=$(timeout 10 ../common/sctp_client 192.168.90.1 41291 $extIP 55004)
if [[ ${out} == *"server1"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) sctp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) sctp [FAILED]"
  print_debug_info
  exit 1
fi

exit
