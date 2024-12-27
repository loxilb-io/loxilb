#!/bin/bash
source ../common.sh
echo k3s-flannel-cluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 5
extIP="123.123.123.1"
echo $extIP

echo "Service Info"
vagrant ssh master1 -c 'sudo kubectl get svc'

print_debug_info() {
  echo "cluster-info"
  vagrant ssh master1 -c 'sudo kubectl get pods -A'
  vagrant ssh master1 -c 'sudo kubectl get svc'
  vagrant ssh master1 -c 'sudo kubectl get nodes'
}

out=$(vagrant ssh host -c "curl -s --connect-timeout 10 http://$extIP:55002")
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) tcp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) tcp [FAILED]"
  print_debug_info
  exit 1
fi

out=$(vagrant ssh host -c "timeout 10 ./udp_client $extIP 55003")
if [[ ${out} == *"Client"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) udp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) udp [FAILED]"
  print_debug_info
  exit 1
fi

#vagrant ssh host -c "socat -v -T10 - sctp:$extIP:55004,bind=192.168.90.9 1> /vagrant/log1.txt 2>&1"
out=$(vagrant ssh host  -c "timeout 10 ./sctp_socat_client 192.168.90.9 0 $extIP 55004")
if [[ ${out} == *"server1"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) sctp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) sctp [FAILED]"
  print_debug_info
  exit 1
fi

#vagrant ssh host -c "socat -v -T10 - sctp:$extIP:57004,bind=192.168.90.9 1> /vagrant/log2.txt 2>&1"
out=$(vagrant ssh host  -c "timeout 10 ./sctp_socat_client 192.168.90.9 0 $extIP 57004")
if [[ ${out} == *"server1"* ]]; then
  echo "k3s-flannel-cluster (kube-loxilb) default-sctp [OK]"
else
  echo "k3s-flannel-cluster (kube-loxilb) default-sctp [FAILED]"
  print_debug_info
  exit 1
fi

exit
