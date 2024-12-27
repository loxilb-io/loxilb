#!/bin/bash
extIP=$(cat /vagrant/extIP)

mode="default"
tcp_port=55002

code=0
echo TCP Service IP: $extIP

ip -6 route list match $extIP | grep $extIP -A 2

echo -e "\n*********************************************"
echo "Testing Service"
echo "*********************************************"
for((i=0;i<20;i++))
do
  out=$(curl -s --connect-timeout 10 http://[$extIP]:$tcp_port)
  if [[ ${out} == *"Welcome to nginx"* ]]; then
    echo -e "dual-stack TCP\t($mode)\t[OK]"
  else
    echo -e "dual-stack TCP\t($mode)\t[FAILED]"
    code=1
  fi
done
