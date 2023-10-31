#!/bin/bash
extIP=$(cat /vagrant/extIP)

mode="onearm"
tcp_port=56002
udp_port=56003
sctp_port=56004

code=0
echo Service IP: $extIP

numECMP=$(ip route list match $extIP | grep $extIP -A 2 | tail -n 2 | wc -l)

ip route list match $extIP | grep $extIP -A 2

if [ $numECMP == "2" ]; then
    echo "Host ECMP route [OK]"
else
    echo "Host ECMP route [NOK]"
fi
echo -e "\n*********************************************"
echo "Testing Service"
echo "*********************************************"
for((i=0;i<20;i++))
do

out=$(curl -s --connect-timeout 10 http://$extIP:$tcp_port)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "K8s-calico-ipvs3 TCP\t($mode)\t[OK]"
else
  echo -e "K8s-calico-ipvs3 TCP\t($mode)\t[FAILED]"
  code=1
fi

out=$(timeout 5 /vagrant/udp_client $extIP $udp_port)
if [[ ${out} == *"Client"* ]]; then
  echo -e "K8s-calico-ipvs3 UDP\t($mode)\t[OK]"
else
  echo -e "K8s-calico-ipvs3 UDP\t($mode)\t[FAILED]"
  code=1
fi

sctp_darn -H 192.168.80.9 -h 20.20.20.1 -p 56004 -s < /vagrant/input > output
#sleep 2
exp="New connection, peer addresses
20.20.20.1:56004"

res=`cat output | grep -A 1 "New connection, peer addresses"`
sudo rm -rf output
if [[ "$res" == "$exp" ]]; then
    #echo $res
    echo -e "K8s-calico-ipvs3 SCTP\t($mode)\t[OK]"
else
    echo -e "K8s-calico-ipvs3 SCTP\t($mode)\t[FAILED]"
    code=1
fi


done
exit $code
