#!/bin/bash
extIP=$(cat /vagrant/extIP)

mode="onearm"
tcp_port=55001
udp_port=55002
sctp_port=55003

code=0
echo Service IP: $extIP

ip route list match $extIP | grep $extIP -A 2

echo -e "\n*********************************************"
echo "Testing Service"
echo "*********************************************"
for((i=0;i<20;i++))
do

out=$(curl -s --connect-timeout 10 http://$extIP:$tcp_port)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "K3s-flannel-incluster-l2 TCP\t($mode)\t[OK]"
else
  echo -e "K3s-flannel-incluster-l2 TCP\t($mode)\t[FAILED]"
  code=1
fi

out=$(timeout 5 /vagrant/udp_client $extIP $udp_port)
if [[ ${out} == *"Client"* ]]; then
  echo -e "K3s-flannel-incluster-l2 UDP\t($mode)\t[OK]"
else
  echo -e "K3s-flannel-incluster-l2 UDP\t($mode)\t[FAILED]"
  code=1
fi

sctp_darn -H 192.168.80.9 -h $extIP -p $sctp_port -s < /vagrant/input > output
#sleep 2
exp="New connection, peer addresses
192.168.80.200:55003"

res=`cat output | grep -A 1 "New connection, peer addresses"`
sudo rm -rf output
if [[ "$res" == "$exp" ]]; then
    #echo $res
    echo -e "K3s-flannel-incluster-l2 SCTP\t($mode)\t[OK]"
else
    echo -e "K3s-flannel-incluster-l2 SCTP\t($mode)\t[FAILED]"
    code=1
fi


done
exit $code
