#!/bin/bash
source ../common.sh
echo cilium-k3s

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "nginx-lb")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find nginx-lb service"
    sleep 1
    continue
  fi 
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP=${strarr[3]}
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

## Any routing updates  ??
sleep 30

echo $extIP
$dexec llb1 loxicmd get lb -o wide
$dexec llb1 loxicmd get ep -o wide

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002) 

if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "cilium-k3s (kube-loxilb) [OK]"
else
  echo "cilium-k3s (kube-loxilb) [FAILED]"
  ## Dump some debug info
  echo "llb1 lb-info"
  $dexec llb1 loxicmd get lb
  echo "llb1 route-info"
  $dexec llb1 ip route
  echo "llb2 lb-info"
  $dexec llb2 loxicmd get lb
  echo "llb2 route-info"
  $dexec llb2 ip route
  echo "r1 route-info"
  $dexec r1 ip route
  exit 1
fi

out=$($hexec user ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo cilium-k3s udp [OK]
else
  echo cilium-k3s udp [FAILED]
  exit 1
fi

exit
