#!/bin/bash
set -e
source ../k3s_common.sh

vagrant destroy -f
vagrant up
#sudo ip route add 123.123.123.1 via 192.168.90.10 || true
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/tcp-onearm-ds.yml'
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/udp-onearm-ds.yml'
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/sctp-onearm-ds.yml'

# kubectl create returns before the images are pulled. Wait for the workloads
# and for kube-loxilb to hand out the external addresses, and record those
# addresses for validation.sh instead of assuming what they will be.
wait_for_daemonset tcp-onearm-ds
wait_for_daemonset udp-onearm-ds
wait_for_daemonset sctp-onearm-ds
wait_for_service_addresses tcp-onearm-svc udp-onearm-svc sctp-onearm-svc

# kube-loxilb reports the address as llb-<ip>; host_validation.sh wants the bare ip.
get_service_address tcp-onearm-svc  | sed 's/^llb-//' > extIP
get_service_address udp-onearm-svc  | sed 's/^llb-//' > extIP1
get_service_address sctp-onearm-svc | sed 's/^llb-//' > extIP2
