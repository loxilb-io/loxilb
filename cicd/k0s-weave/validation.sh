#!/bin/bash
source ../common.sh
echo cluster-k0s

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(sudo k0s kubectl get svc | grep "udp-lb1")
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
  echo "**** k0s svc info ****"
  sudo k0s kubectl get svc
  echo "**** k0s pods info ****"
  sudo k0s kubectl get pods -A
  echo "**** k0s endpoints info ****"
  sudo k0s kubectl get endpoints

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
  echo "cluster-k0s (tcp)  [OK]"
else
  code=1
fi

out=$($hexec user timeout 10 ../common/udp_client $extIP 55003)
if [[ ${out} == *"Client"* ]]; then
  echo "cluster-k0s (udp)  [OK]"
else
  echo "cluster-k0s (udp)  [FAILED]"
  code=1
fi

out=$($hexec user timeout 10 ../common/sctp_client 1.1.1.1 34951 $extIP 55004)
if [[ ${out} == *"server1"* ]]; then
  echo "cluster-k0s (sctp) [OK]"
else
  echo "cluster-k0s (sctp) [FAILED]"
  code=1
fi

out=$($hexec user timeout 10 ../common/sctp_client 1.1.1.1 0 $extIP 55005)
if [[ ${out} == *"server1"* ]]; then
  echo "cluster-k0s (sctp2) [OK]"
else
  echo "cluster-k0s (sctp2) [FAILED]"
  code=1
fi

out=$($hexec user timeout 10 ../common/udp_client $extIP 55006)
if [[ ${out} == *"Client"* ]]; then
  echo "cluster-k0s (udp2)  [OK]"
else
  echo "cluster-k0s (udp2)  [FAILED]"
  code=1  
fi

if [[ $code -eq 1 ]]; then
  echo "cluster-k0s failed"
  exit 1
fi

exit

