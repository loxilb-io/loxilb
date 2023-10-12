#!/bin/bash
source ../common.sh
echo k8s-calico-incluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

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
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

## Any routing updates  ??
sleep 30

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
mode=( "fullnat" )
tcp_port=( 57002 )
udp_port=( 57003 )
sctp_port=( 57004 )
code=0
for ((i=0;i<1;i++)); do
out=$(vagrant ssh host -c "curl -s --connect-timeout 10 http://$extIP:${tcp_port[i]}")
echo $out
if [[ ${out} == *"nginx"* ]]; then
  echo -e "K8s-calico-incluster TCP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico-incluster TCP\t(${mode[i]})\t[FAILED]"
  code=1
fi

out=$(vagrant ssh host -c "timeout 5 ./udp_client $extIP ${udp_port[i]}")
if [[ ${out} == *"Client"* ]]; then
  echo -e "K8s-calico-incluster UDP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico-incluster UDP\t(${mode[i]})\t[FAILED]"
  code=1
fi

out=$(vagrant ssh host -c "socat -T10 - SCTP:$extIP:${sctp_port[i]},bind=192.168.90.9")
if [[ ${out} == *"server"* ]]; then
  echo -e "K8s-calico-incluster SCTP\t(${mode[i]})\t[OK]"
else
  echo -e "K8s-calico-incluster SCTP\t(${mode[i]})\t[FAILED]"
  code=1
fi
done

mode=( "default" )
sctp_port=( 55004 )
code=0

out=$(vagrant ssh host -c "socat -T10 - SCTP:$extIP:${sctp_port[0]},bind=192.168.90.9")
if [[ ${out} == *"server"* ]]; then
  echo -e "K8s-calico-incluster SCTP\t(${mode[0]})\t[OK]"
else
  echo -e "K8s-calico-incluster SCTP\t(${mode[0]})\t[FAILED]"
  code=1
fi

exit $code
