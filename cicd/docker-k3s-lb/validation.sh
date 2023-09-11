#!/bin/bash
source ../common.sh
echo docker-k3s-lb

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 30
extIP="192.168.163.247"
echo $extIP

echo "Service Info"
vagrant ssh llb1 -c 'sudo kubectl get svc'
echo "LB Info"
vagrant ssh llb1 -c 'sudo docker exec -i loxilb loxicmd get lb -o wide'
echo "EP Info"
vagrant ssh llb1 -c 'sudo docker exec -i loxilb loxicmd get ep -o wide'

print_debug_info() {
  echo "llb1 route-info"
  vagrant ssh llb1 -c 'ip route'
  vagrant ssh llb1 -c 'sudo kubectl get pods -A'
  vagrant ssh llb1 -c 'sudo kubectl get svc'
  vagrant ssh llb1 -c 'sudo kubectl get nodes'
}

sctp_darn -H 192.168.163.1 -h 192.168.163.247 -p 55003 -s < input > output
sleep 5
exp="New connection, peer addresses
192.168.163.247:55003"

res=`cat output | grep -A 1 "New connection, peer addresses"`
echo "Result"
echo $res
echo "Expected"
echo $exp
sudo rm -rf output
if [[ "$res" == "$exp" ]]; then
    echo $res
    echo "docker-k3s-lb SCTP service sctp-lb (loxilb) [OK]"
else
    echo "docker-k3s-lb SCTP service sctp-lb (loxilb) [NOK]"
    print_debug_info 
    exit 1
fi
