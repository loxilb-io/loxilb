#!/bin/bash
source ../common.sh
echo k3s-sctpmh-seagull

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '
sleep 10

extIP=""
for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'sudo kubectl get svc' 2> /dev/null | grep "multus-seagull-service")
  echo $extLB
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find multus-seagull-service"
    sleep 1
    continue
  fi
  if [[ ${strarr[3]} != *"none"* && ${strarr[3]} != *"pending"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

echo Service IP : $extIP
echo $extIP > extIP

echo "Service Info"
vagrant ssh master -c 'sudo kubectl get svc'
echo "LB Info"
vagrant ssh loxilb1 -c 'sudo docker exec -i loxilb loxicmd get lb -o wide'
echo "EP Info"
vagrant ssh loxilb1 -c 'sudo docker exec -i loxilb loxicmd get ep -o wide'

print_debug_info() {
  echo "llb1 route-info"
  vagrant ssh loxilb1 -c 'ip route'
  vagrant ssh master -c 'sudo kubectl get pods -A'
  vagrant ssh master -c 'sudo kubectl get svc'
  vagrant ssh master -c 'sudo kubectl get nodes'
}

code=0

if [[ -z "$extIP" ]]; then
  echo "k3s-sctpmh-seagull: multus-seagull-service got no external IP [FAILED]"
  print_debug_info
  rm -f extIP
  exit 1
fi

# The pod must come up with the macvlan secondary interface, not just the
# default flannel one. Without it there is nothing for loxilb to multihome to.
ifaces=$(vagrant ssh master -c 'sudo kubectl exec seagull-pod-01 -- ls /sys/class/net' 2> /dev/null | tr -d '\r')
if [[ "$ifaces" == *"net1"* ]]; then
  echo "k3s-sctpmh-seagull multus secondary interface  [OK]"
else
  echo "k3s-sctpmh-seagull multus secondary interface  [NOK]"
  echo "Expected : net1 among the pod interfaces"
  echo "Received : $ifaces"
  code=1
fi

# loxilb must program the SCTP rule against the pod's macvlan address
# (4.0.6.0/24), not against its flannel address.
lbRule=$(vagrant ssh loxilb1 -c 'sudo docker exec -i loxilb loxicmd get lb -o wide' 2> /dev/null | grep "multus-seagull-service")
if [[ "$lbRule" == *"sctp"* && "$lbRule" == *"4.0.6."* ]]; then
  echo "k3s-sctpmh-seagull SCTP multihoming LB rule  [OK]"
else
  echo "k3s-sctpmh-seagull SCTP multihoming LB rule  [NOK]"
  echo "Expected : an sctp rule with a 4.0.6.0/24 endpoint"
  echo "Received : $lbRule"
  code=1
fi

if [[ $code == 0 ]]; then
  echo "k3s-sctpmh-seagull validation [OK]"
else
  print_debug_info
  echo "k3s-sctpmh-seagull validation [NOK]"
fi

rm -f extIP
exit $code
