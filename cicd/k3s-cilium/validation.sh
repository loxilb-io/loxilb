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
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

## Any routing updates  ??
sleep 30

code=0
echo $extIP
echo "loxilb info"
$dexec llb1 loxicmd get lb -o wide
$dexec llb1 loxicmd get ep -o wide
echo "k3s info"
kubectl $KUBECONFIG get endpoints
kubectl $KUBECONFIG get pods -A
kubectl $KUBECONFIG get svc

debug_output () {
  ## Dump some debug info
  lsb_release -a
  echo "sys route-info"
  ip route
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
}

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "cilium-k3s (kube-loxilb) tcp [OK]"
else
  echo "cilium-k3s (kube-loxilb) tcp [FAILED]"
  debug_output
  code=1
fi

out=$($hexec user timeout 10 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "cilium-k3s (kube-loxilb) udp [OK]"
else
  echo "cilium-k3s (kube-loxilb) udp [FAILED]"
  debug_output
  code=1
fi

if [[ $code == 0 ]]
then
  echo SCENARIO-k3s-cilium [OK]
else
  echo SCENARIO-k3s-cilium [FAILED]
  exit 1
fi

exit
