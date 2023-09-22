#!/bin/bash
source ../common.sh
echo k8s-calico-ipvs3

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'kubectl get svc' 2> /dev/null | grep "tcp-lb-onearm")
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
vagrant ssh llb1 -c 'sudo docker exec -it loxilb loxicmd get lb -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\nEP List"
echo "******************************************************************************"
vagrant ssh llb1 -c 'sudo docker exec -it loxilb loxicmd get ep -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\nTEST RESULTS"
echo "******************************************************************************"

vagrant ssh host -c 'sudo /vagrant/host_validation.sh' 2> /dev/null
sudo rm extIP
