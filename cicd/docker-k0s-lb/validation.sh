#!/bin/bash
source ../common.sh
echo docker-k0s-lb

sleep 30
extIP="192.168.82.100"
echo $extIP

echo "Service Info"
vagrant ssh llb1 -c 'sudo k0s kubectl get svc'
echo "LB Info"
vagrant ssh llb1 -c 'sudo docker exec -i loxilb loxicmd get lb -o wide'
echo "EP Info"
vagrant ssh llb1 -c 'sudo docker exec -i loxilb loxicmd get ep -o wide'

print_debug_info() {
  echo "llb1 route-info"
  vagrant ssh llb1 -c 'ip route'
  vagrant ssh llb1 -c 'sudo k0s kubectl get pods -A'
  vagrant ssh llb1 -c 'sudo k0s kubectl get svc'
  vagrant ssh llb1 -c 'sudo k0s kubectl get nodes'
}

out=$(curl -s --connect-timeout 10 http://$extIP:56002)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "k0s-docker (kube-loxilb) tcp [OK]"
else
  echo "k0s-docker (kube-loxilb) tcp [FAILED]"
  print_debug_info
  exit 1
fi
