#!/bin/bash
source ../common.sh
echo calico-k3s

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "nginx-lb1")
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

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002) 

if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "calico-k3s (kube-loxilb) tcp [OK]"
else
  echo "calico-k3s (kube-loxilb) tcp [FAILED]"
  code=1
fi

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "udp-lb1")
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

out=$($hexec user timeout 30 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "calico-k3s (kube-loxillb) udp [OK]"
else
  echo "calico-k3s (kube-loxillb) udp [FAILED]"
  code=1
fi

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "sctp-lb1")
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

out=$($hexec user timeout 30 ../common/sctp_socat_client 1.1.1.1 41291 $extIP 55004)
if [[ ${out} == *"server1"* ]]; then
  echo "calico-k3s (kube-loxillb) sctp [OK]"
else
  echo "calico-k3s (kube-loxillb) sctp [FAILED]"
  code=1
fi

if [[ $code -eq 1 ]]; then
  echo "calico-k3s failed"
  exit 1
fi

exit
