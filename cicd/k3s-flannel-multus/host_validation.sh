#!/bin/bash
extIP=$(cat /vagrant/extIP)

tcp_port=55002
sctp_port=55002

code=0
echo Service IP: $extIP

echo "*********************************************"
for((i=0;i<20;i++))
do
out=$(curl -s --connect-timeout 10 http://$extIP:$tcp_port)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo -e "K3s-multus TCP\t($mode)\t[OK]"
else
  echo -e "K3s-multus TCP\t($mode)\t[FAILED]"
  code=1
fi
done

for((i=0;i<20;i++))
do
stdbuf -oL sctp_darn -H 0.0.0.0 -h $extIP -p $sctp_port -s < /vagrant/input > output
#sleep 1
exp="New connection, peer addresses
4.0.5.2:55002
4.0.3.1:55002
4.0.4.1:55002"

res=`cat output | grep -A 3 "New connection, peer addresses"`
sudo rm -rf output
if [[ "$res" == "$exp" ]]; then
    #echo $res
    echo "K3s-multus SCTP Multihoming service  [OK]"
else
    echo "K3s-multus SCTP Multihoming service  [NOK]"
    echo "Expected : $exp"
    echo "Received : $res"
    exit 1
fi
done
exit $code
