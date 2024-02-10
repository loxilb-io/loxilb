#!/bin/bash
extIP=$(cat /vagrant/extIP)

tcp_port=55002

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
exit $code
