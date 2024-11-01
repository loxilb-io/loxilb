#!/bin/bash
source ../common.sh
echo k3s-loxi-gwapi

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

#sleep 45

echo -e "\n\nGateway Info\n"
vagrant ssh master -c 'sudo kubectl get gateway -A' 2> /dev/null
echo -e "\n\nTCPRoute Info\n"
vagrant ssh master -c 'sudo kubectl get tcproute -A' 2> /dev/null
echo -e "\n\nUDPRoute Info\n"
vagrant ssh master -c 'sudo kubectl get udproute -A' 2> /dev/null
echo -e "\n\nHTTPRoute Info\n"
vagrant ssh master -c 'sudo kubectl get httproute -A' 2> /dev/null
echo -e "\n\nService Info\n"
vagrant ssh master -c 'sudo kubectl get svc -A' 2> /dev/null
echo -e "\n\nEP Info\n"
vagrant ssh master -c 'sudo kubectl get ep -A' 2> /dev/null
echo -e "\n\nIngress Info\n"
vagrant ssh master -c 'sudo kubectl get ingress -A' 2> /dev/null
echo -e "\n\nLB service Info\n"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get lb -o wide' 2> /dev/null
echo -e "\n\nLB ep Info\n"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get ep -o wide' 2> /dev/null
echo -e "\n\n"
out=$(curl -s http://192.168.80.90:21818)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "k3s-loxi-gwapi tcpRoute \t\t[OK]"
else
  echo -e "k3s-loxi-gwapi tcpRoute \t\t[FAILED]"
  code=1
fi

out=$(timeout 10 ../common/udp_client 192.168.80.90 21819)
if [[ ${out} == *"Client"* ]]; then
  echo -e "k3s-loxi-gwapi udpRoute \t\t[OK]"
else
  echo -e "k3s-loxi-gwapi udpRoute \t\t[FAILED]"
  code=1
fi

out=$(curl -s --connect-timeout 30 -H "Application/json" -H "Content-type: application/json" -H "HOST: test.loxilb.gateway.http" http://192.168.80.90:80)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "k3s-loxi-gwapi httpRoute \t\t[OK]"
else
  echo -e "k3s-loxi-gwapi httpRoute \t\t[FAILED]"
  code=1
fi

out=$(curl -s --connect-timeout 30 -H "Application/json" -H "Content-type: application/json" -H "HOST: test.loxilb.gateway.https" --insecure https://192.168.80.90:443)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "k3s-loxi-gwapi httpRoute(https) \t[OK]"
else
  echo -e "k3s-loxi-gwapi httpRoute(https) \t[FAILED]"
  code=1
fi

exit $code
