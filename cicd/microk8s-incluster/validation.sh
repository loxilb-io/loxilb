#!/bin/bash
source ../common.sh
echo microk8s-incluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh mk8s-node1 -c 'sudo microk8s kubectl get svc' 2> /dev/null | grep tcp-lb"")
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
echo $extIP

echo -e "\nEnd Points List"
echo "******************************************************************************"
vagrant ssh mk8s-node1 -c 'sudo microk8s kubectl get endpoints -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh mk8s-node1 -c 'sudo microk8s kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nPod List"
echo "******************************************************************************"
vagrant ssh mk8s-node1 -c 'sudo microk8s kubectl get pods -A' 2> /dev/null
echo "******************************************************************************"

echo -e "\nTEST RESULTS"
echo "******************************************************************************"

echo -e "Command: curl --connect-time 10 http://${extIP}:56002'\n"
res=`curl -s --connect-time 10 http://${extIP}:56002`
echo "Result"
echo $res
if [[ "$res" == *"Welcome to nginx"* ]]; then
    echo -e "\n\nmicrok8s-incluster TCP service (loxilb) [OK]"
else
    echo -e "\n\nmicrok8s-incluster TCP service (loxilb) [NOK]"
    exit 1
fi

