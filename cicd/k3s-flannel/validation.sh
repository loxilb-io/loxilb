#!/bin/bash
source ../common.sh
echo cluster-k3s

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

echo "ExternalIP $extIP"

print_debug_info() {
  ## Dump some debug info
  echo "**** k3s svc info ****"
  sudo kubectl $KUBECONFIG get svc
  echo "**** k3s pods info ****"
  sudo kubectl $KUBECONFIG get pods -A

  echo "**** llb1 lb-info ****"
  $dexec llb1 loxicmd get lb -o wide
  echo "**** loxilb ep-info ****"
  $dexec llb1 loxicmd get ep -o wide
  echo "**** llb1 route-info ****"
  $dexec llb1 ip route

  echo "**** llb2 lb-info ****"
  $dexec llb2 loxicmd get lb -o wide
  echo "**** loxilb ep-info ****"
  $dexec llb1 loxicmd get ep -o wide
  echo "**** llb2 route-info ****"
  $dexec llb2 ip route

  echo "**** r1 route-info ****"
  $dexec r1 ip route
}

code=0
print_debug_info

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:80) 
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "cluster-k3s (ccm) [OK]"
else
  echo "cluster-k3s (ccm) [FAILED]"
  code=1
fi

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002) 
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "cluster-k3s (kube-loxilb) tcp [OK]"
else
  echo "cluster-k3s (kube-loxilb) tcp [FAILED]"
  code=1
fi

out=$($hexec user timeout 30 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "cluster-k3s (kube-loxilb) udp [OK]"
else
  echo "cluster-k3s (kube-loxilb) udp [FAILED]"
  code=1
fi

if [[ $code -eq 1 ]]; then
  echo "cluster-k3s failed"
  exit 1
fi

exit
