#!/bin/bash
source ../common.sh
echo k3s-multus

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '
sleep 10

extLB=""
for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'sudo kubectl get svc' 2> /dev/null | grep "multus-service")
  echo $extLB
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find multus-service"
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

echo Service IP : $extIP
echo $extIP > extIP

echo "Service Info"
vagrant ssh master -c 'sudo kubectl get svc'
echo "LB Info"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get lb -o wide'
echo "EP Info"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get ep -o wide'

print_debug_info() {
  echo "llb1 route-info"
  vagrant ssh loxilb -c 'ip route'
  vagrant ssh master -c 'sudo kubectl get pods -A'
  vagrant ssh master -c 'sudo kubectl get svc'
  vagrant ssh master -c 'sudo kubectl get nodes'
}

vagrant ssh bastion -c 'sudo /vagrant/host_validation.sh' 2> /dev/null
sudo rm extIP
