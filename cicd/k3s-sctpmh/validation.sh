#!/bin/bash
source ../common.sh
source ../k3s_common.sh

echo "cluster-k3s: TCP & SCTP Multihoming combined"

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "nginx-lb1")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find nginx-lb service"
    sleep 1
    continue
  fi 
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    port=${strarr[4]}
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

## Any routing updates  ??
#sleep 30
echo $extIP

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002) 
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "cluster-k3s TCP service nginx-lb (kube-loxilb) [OK]"
else
  echo "cluster-k3s TCP service nginx-lb (kube-loxilb) [FAILED]"
  ## Dump some debug info
  echo "llb1 lb-info"
  $dexec llb1 loxicmd get lb
  echo "llb1 route-info"
  $dexec llb1 ip route
  echo "llb2 lb-info"
  $dexec llb2 loxicmd get lb
  echo "llb2 route-info"
  $dexec llb2 ip route
  echo "r1 route-info"
  $dexec r1 ip route
  exit 1
fi

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "sctp-lb1")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find sctp-lb1 service"
    sleep 1
    continue
  fi 
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    port=${strarr[4]}
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

echo "SCTP Multihoming service sctp-lb1 -> $extIP:$port"

$hexec user sctp_darn -H 1.1.1.1 -h 123.123.123.1 -p 55003 -s < input > output
sleep 5
exp="New connection, peer addresses
123.123.123.1:55003
124.124.124.1:55003
125.125.125.1:55003"

res=`cat output | grep -A 3 "New connection, peer addresses"`
sudo rm -rf output
if [[ "$res" == "$exp" ]]; then
    echo $res
    echo "cluster-k3s SCTP Multihoming service sctp-lb1 (kube-loxilb) [OK]"
else
    echo "cluster-k3s SCTP Multihoming service sctp-lb1 (kube-loxilb) [NOK]"
    echo "Expected : $exp"
    echo "Received : $res"
    ## Dump some debug info
    echo "system route-info"
    ip route
    echo "system ipables"
    sudo iptables -n -t nat -L -v  |grep sctp
    echo "llb1 lb-info"
    $dexec llb1 loxicmd get lb
    echo "llb1 ep-info"
    $dexec llb1 loxicmd get ep
    echo "llb1 bpf-info"
    $dexec llb1 ntc filter show dev eth0 ingress
    echo "llb1 route-info"
    $dexec llb1 ip route
    echo "llb2 lb-info"
    $dexec llb2 loxicmd get lb
    echo "llb2 route-info"
    $dexec llb2 ip route
    echo "r1 route-info"
    $dexec r1 ip route
    echo "BFP trace -- "
    sudo timeout 5 cat  /sys/kernel/debug/tracing/trace_pipe
    sudo killall -9 cat
    echo "BFP trace -- "
    exit 1
fi

## Check delete and readd service
kubectl $KUBECONFIG delete -f nginx-svc-lb1.yml
sleep 10
kubectl $KUBECONFIG apply -f nginx-svc-lb1.yml
sleep 10

# Wait for cluster to be ready
wait_cluster_ready_full

for((i=0; i<120; i++))
do
  extLB=$(sudo kubectl $KUBECONFIG get svc | grep "nginx-lb1")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find nginx-lb service"
    sleep 1
    continue
  fi
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    port=${strarr[4]}
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

out=$($hexec user curl -s --connect-timeout 10 http://$extIP:55002)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "cluster-k3s TCP service nginx-lb del+add (kube-loxilb) [OK]"
else
  echo "cluster-k3s TCP service nginx-lb del+add (kube-loxilb) [FAILED]"
  ## Dump some debug info
  echo "llb1 lb-info"
  $dexec llb1 loxicmd get lb
  echo "llb1 route-info"
  $dexec llb1 ip route
  echo "llb2 lb-info"
  $dexec llb2 loxicmd get lb
  echo "llb2 route-info"
  $dexec llb2 ip route
  echo "r1 route-info"
  $dexec r1 ip route
  exit 1
fi

