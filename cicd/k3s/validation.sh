#!/bin/bash
source ../common.sh
echo cluster-k3s

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl get svc | grep "nginx-lb")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find nginx-lb service"
    sleep 1
    continue
  fi 
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP=${strarr[3]}
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

## Any routing updates  ??
sleep 30

echo $extIP
out=$($hexec user curl -s --connect-timeout 10 http://$extIP:80) 

if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo cluster-k3s [OK]
else
  echo cluster-k3s [FAILED]
  exit 1
fi
