#!/bin/bash
source ../common.sh
echo calico-k3s-dual-cluster

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

echo "ExternalIP $extIP"

print_debug_info() {
  ## Dump some debug info
  echo -e "\n\nDEBUG INFO"
  echo "*************************************************************************"
  echo -e "\n**** k3s svc info ****"
  sudo kubectl $KUBECONFIG get svc
  echo -e "\n**** k3s pods info ****"
  sudo kubectl $KUBECONFIG get pods -A

  echo -e "\n**** llb1 lb-info ****"
  $dexec llb1 loxicmd get lb -o wide
  echo -e "\n**** loxilb ep-info ****"
  $dexec llb1 loxicmd get ep -o wide
  echo -e "\n**** llb1 route-info ****"
  $dexec llb1 ip route

  echo -e "\n**** r1 route-info ****"
  $dexec r1 ip route
  echo "*************************************************************************"
}

code=0
print_debug_info

echo -e "\n\nTEST RESULTS"
echo "*********************************************************************************"
out=$($hexec user curl -s --connect-timeout 10 http://$extIP:80) 
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "calico-k3s-dual-cluster (ccm) [OK]"
else
  echo "calico-k3s-dual-cluster (ccm) [FAILED]"
  code=1
fi

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002) 

if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "calico-k3s-dual-cluster (kube-loxilb) tcp [OK]"
else
  echo "calico-k3s-dual-cluster (kube-loxilb) tcp [FAILED]"
  code=1
fi

out=$($hexec user timeout 30 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "calico-k3s-dual-cluster (kube-loxillb) udp [OK]"
else
  echo "calico-k3s-dual-cluster (kube-loxillb) udp [FAILED]"
  code=1
fi

out=$($hexec user timeout 30 ../common/sctp_client 1.1.1.1 41291 $extIP 55004)
if [[ ${out} == *"server1"* ]]; then
  echo "calico-k3s-dual-cluster (kube-loxillb) sctp [OK]"
else
  echo "calico-k3s-dual-cluster (kube-loxillb) sctp [FAILED]"
  code=1
fi

if [[ $code -eq 1 ]]; then
  echo "calico-k3s-dual-cluster failed"
  exit 1
fi

exit
