#!/bin/bash
source ../common.sh
echo k8s-calico

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'kubectl get svc' 2> /dev/null | grep "tcp-lb-default")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find tcp-lb service"
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
#sleep 30

echo Service IP : $extIP
echo -e "\nEnd Points List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get endpoints -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nPod List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get pods -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nLB List"
echo "******************************************************************************"
vagrant ssh loxilb -c 'sudo docker exec -it loxilb loxicmd get lb -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\nEP List"
echo "******************************************************************************"
vagrant ssh loxilb -c 'sudo docker exec -it loxilb loxicmd get ep -o wide' 2> /dev/null
echo "******************************************************************************"

echo -e "\nTEST RESULTS"
echo "******************************************************************************"
mode=( "default" "onearm" "fullnat" )
tcp_port=( 55002 56002 57002 )
udp_port=( 55003 56003 57003 )
sctp_port=( 55004 56004 57004 )
code=0
for ((i=0;i<=2;i++)); do
out=$(curl -s --connect-timeout 10 http://$extIP:${tcp_port[i]})
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "K8s-calico TCP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico TCP\t(${mode[i]})\t[FAILED]"
  ## Dump some debug info
  echo "llb1 lb-info"
  vagrant ssh loxilb -c 'sudo docker exec -it llb1 loxicmd get lb -o wide' 2> /dev/null
  echo "llb1 route-info"
  vagrant ssh loxilb -c 'sudo docker exec -it llb1 ip route' 2> /dev/null
  code=1
fi

out=$(timeout 5 ../common/udp_client $extIP ${udp_port[i]})
if [[ ${out} == *"Client"* ]]; then
  echo -e "K8s-calico UDP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico UDP\t(${mode[i]})\t[FAILED]"
  ## Dump some debug info
  echo "llb1 lb-info"
  vagrant ssh loxilb -c 'sudo docker exec -it llb1 loxicmd get lb -o wide' 2> /dev/null
  echo "llb1 route-info"
  vagrant ssh loxilb -c 'sudo docker exec -it llb1 ip route' 2> /dev/null
  code=1
fi

out=$(timeout 5 ../common/sctp_socat_client 192.168.90.1 34951 $extIP ${sctp_port[i]})
if [[ ${out} == *"server1"* ]]; then
  echo -e "K8s-calico SCTP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico SCTP\t(${mode[i]})\t[FAILED]"
  ## Dump some debug info
  echo "llb1 lb-info"
  vagrant ssh loxilb -c 'sudo docker exec -it llb1 loxicmd get lb -o wide' 2> /dev/null
  echo "llb1 route-info"
  vagrant ssh loxilb -c 'sudo docker exec -it llb1 ip route' 2> /dev/null
  code=1
fi
done
exit $code
