#!/bin/bash
source ../common.sh
echo k3s-loxi-ingress

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 45

echo "Service Info"
vagrant ssh master -c 'sudo kubectl get svc -A'
echo "Ingress Info"
vagrant ssh master -c 'sudo kubectl get ingress -A'
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

out=$(curl -s --connect-timeout 30 -H "Application/json" -H "Content-type: application/json" -H "HOST: loxilb.io" --insecure https://192.168.80.9:443)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "k3s-loxi-ingress tcp [OK]"
else
  echo "k3s-loxi-ingress tcp [FAILED]"
  print_debug_info
  exit 1
fi

exit
