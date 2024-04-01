#!/bin/bash
source ../common.sh
echo k3s-ext-ip

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

set -eo pipefail
# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'sudo kubectl get svc' 2> /dev/null | grep "nginx")
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
echo $extIP > extIP

echo -e "\nEnd Points List"
echo "******************************************************************************"
vagrant ssh master -c 'sudo kubectl get endpoints -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh master -c 'sudo kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nPod List"
echo "******************************************************************************"
vagrant ssh master -c 'sudo kubectl get pods -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nLB List"
echo "******************************************************************************"
vagrant ssh master -c 'sudo sudo docker exec -it loxilb loxicmd get lb -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\nEP List"
echo "******************************************************************************"
vagrant ssh master -c 'sudo docker exec -it loxilb loxicmd get ep -o wide' 2> /dev/null
echo "******************************************************************************"

echo -e "\nTEST RESULTS"
echo "******************************************************************************"

echo -e "\n\nCommand: curl --connect-time 10 http://20.20.20.1:55002'\n\n"
vagrant ssh host -c 'curl --connect-time 10 http://20.20.20.1:55002' 2> /dev/null
echo -e "\n\n\nConnecting external EP service from the pod\n\n"
echo "sudo kubectl exec -it nginx-test -- curl 20.20.20.1:8000\n"
vagrant ssh master -c 'sudo kubectl exec -it nginx-test -- curl 20.20.20.1:8000' 2> /dev/null
