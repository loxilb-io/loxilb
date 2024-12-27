#!/bin/bash
source ../common.sh
echo k8s-calico-incluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '
alloc=0
for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'kubectl get svc' 2> /dev/null | grep "tcp-lb-fullnat")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find tcp-lb-fullnat"
    sleep 1
    continue
  fi 
  if [[ ${strarr[3]} != *"none"* || ${strarr[3]} != *"pending"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    alloc=1
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

if [[ $alloc != 1 ]]; then
  echo "No external LB allocated. Check kube-loxilb and loxilb logs"
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
  exit 1
fi

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

echo -e "\nTEST RESULTS"
echo "******************************************************************************"
mode=( "onearm" "fullnat" )
tcp_port=( 56002 57002 )
udp_port=( 56003 57003 )
sctp_port=( 56004 57004 )
code=0

for ((i=0;i<=1;i++)); do
out=$(vagrant ssh host -c "curl -s --connect-timeout 10 http://$extIP:${tcp_port[i]}" 2> /dev/null)
#echo $out
if [[ ${out} == *"nginx"* ]]; then
  echo -e "K8s-calico-incluster TCP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico-incluster TCP\t(${mode[i]})\t[FAILED]"
  code=1
fi

out=$(vagrant ssh host -c "timeout 5 /vagrant/udp_client $extIP ${udp_port[i]}" 2> /dev/null)
#echo $out
if [[ ${out} == *"Client"* ]]; then
  echo -e "K8s-calico-incluster UDP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico-incluster UDP\t(${mode[i]})\t[FAILED]"
  code=1
fi

out=$(vagrant ssh host -c "socat -T10 - SCTP:$extIP:${sctp_port[i]}" 2> /dev/null)
#echo $out
if [[ ${out} == *"server"* ]]; then
  echo -e "K8s-calico-incluster SCTP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico-incluster SCTP\t(${mode[i]})\t[FAILED]"
  code=1
fi
done

exit $code
