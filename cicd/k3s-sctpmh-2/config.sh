#!/bin/bash

source ../common.sh
source ../k3s_common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1 --with-bgp yes --bgp-config $(pwd)/llb1_gobgp_config
spawn_docker_host --dock-type loxilb --dock-name llb2 --with-bgp yes --bgp-config $(pwd)/llb2_gobgp_config
spawn_docker_host --dock-type host --dock-name ep1
spawn_docker_host --dock-type host --dock-name ep2
spawn_docker_host --dock-type host --dock-name ep3
spawn_docker_host --dock-type host --dock-name r1 --with-bgp yes --bgp-config $(pwd)/r1_config
spawn_docker_host --dock-type host --dock-name r2 --with-bgp yes --bgp-config $(pwd)/r2_config
spawn_docker_host --dock-type host --dock-name r3 --with-bgp yes --bgp-config $(pwd)/r3_config
spawn_docker_host --dock-type host --dock-name sw1
spawn_docker_host --dock-type host --dock-name user

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts user r1
connect_docker_hosts r1 sw1
connect_docker_hosts sw1 llb1
connect_docker_hosts user r3
connect_docker_hosts r3 sw1
connect_docker_hosts sw1 llb2
connect_docker_hosts llb1 r2
connect_docker_hosts llb2 r2
connect_docker_hosts r2 ep1
connect_docker_hosts r2 ep2
connect_docker_hosts r2 ep3

create_docker_host_cnbridge --host1 sw1 --host2 llb1
create_docker_host_cnbridge --host1 sw1 --host2 llb2
create_docker_host_cnbridge --host1 sw1 --host2 r1
create_docker_host_cnbridge --host1 sw1 --host2 r3

## Make network for k3s connectivity
sudo ip link add ellb1sys type veth peer name esysllb1
sleep 3
sudo ip link set ellb1sys netns llb1
sleep 3
sudo ip -n llb1 link set ellb1sys up
sudo ip -n llb1 addr add 12.12.12.1/24 dev ellb1sys
sudo ip link set esysllb1 up
sudo ip addr add 12.12.12.254/24 dev esysllb1

sudo ip link add ellb2sys type veth peer name esysllb2
sleep 3
sudo ip link set ellb2sys netns llb2
sleep 3
sudo ip -n llb2 link set ellb2sys up
sudo ip -n llb2 addr add 14.14.14.1/24 dev ellb2sys
sudo ip link set esysllb2 up
sudo ip addr add 14.14.14.254/24 dev esysllb2

#node1 config
config_docker_host --host1 user --host2 r1 --ptype phy --addr 1.1.1.1/24 --gw 1.1.1.254
config_docker_host --host1 r1 --host2 user --ptype phy --addr 1.1.1.254/24
create_docker_host_vlan --host1 llb1 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 llb2 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 r1 --host2 sw1 --id 11 --ptype untagged
create_docker_host_vlan --host1 r3 --host2 sw1 --id 11 --ptype untagged
config_docker_host --host1 r1 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.254/24 --gw 11.11.11.11
config_docker_host --host1 r3 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.253/24 --gw 11.11.11.11
config_docker_host --host1 llb1 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.1/24 --gw 11.11.11.254
config_docker_host --host1 llb2 --host2 sw1 --ptype vlan --id 11 --addr 11.11.11.2/24 --gw 11.11.11.254
config_docker_host --host1 user --host2 r3 --ptype phy --addr 2.2.2.1/24 --gw 2.2.2.254
config_docker_host --host1 r3 --host2 user --ptype phy --addr 2.2.2.254/24

create_docker_host_vlan --host1 llb1 --host2 r2 --id 10 --ptype untagged
config_docker_host --host1 llb1 --host2 r2 --ptype vlan --id 10 --addr 10.10.10.1/24
create_docker_host_vlan --host1 llb2 --host2 r2 --id 10 --ptype untagged
config_docker_host --host1 llb2 --host2 r2 --ptype vlan --id 10 --addr 10.10.10.2/24

create_docker_host_vlan --host1 r2 --host2 llb1 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 llb2 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep1 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep2 --id 10 --ptype untagged
create_docker_host_vlan --host1 r2 --host2 ep3 --id 10 --ptype untagged
config_docker_host --host1 r2 --host2 llb1 --ptype vlan --id 10 --addr 10.10.10.254/24

create_docker_host_vlan --host1 r2 --host2 ep1 --id 31 --ptype untagged
config_docker_host --host1 r2 --host2 ep1 --ptype vlan --id 31 --addr 31.31.31.254/24

create_docker_host_vlan --host1 ep1 --host2 r2 --id 31 --ptype untagged
config_docker_host --host1 ep1 --host2 r2 --ptype vlan --id 31 --addr 31.31.31.1/24 --gw 31.31.31.254

create_docker_host_vlan --host1 r2 --host2 ep2 --id 32 --ptype untagged
config_docker_host --host1 r2 --host2 ep2 --ptype vlan --id 32 --addr 32.32.32.254/24

create_docker_host_vlan --host1 ep2 --host2 r2 --id 32 --ptype untagged
config_docker_host --host1 ep2 --host2 r2 --ptype vlan --id 32 --addr 32.32.32.1/24 --gw 32.32.32.254

create_docker_host_vlan --host1 r2 --host2 ep3 --id 33 --ptype untagged
config_docker_host --host1 r2 --host2 ep3 --ptype vlan --id 33 --addr 33.33.33.254/24

create_docker_host_vlan --host1 ep3 --host2 r2 --id 33 --ptype untagged
config_docker_host --host1 ep3 --host2 r2 --ptype vlan --id 33 --addr 33.33.33.1/24 --gw 33.33.33.254

$hexec user ip route change default via 1.1.1.254

##Pod networks
$hexec r1 ip route add 20.20.20.1/32 via 11.11.11.11
#add_route llb1 1.1.1.0/24 11.11.11.254
#add_route llb2 1.1.1.0/24 11.11.11.254

# Route back to user
#sudo ip route add 11.11.11.0/24 via 12.12.12.1
#sudo ip route add 1.1.1.1 via 12.12.12.1
#sudo ip route add 123.123.123.1 via 12.12.12.1
#sudo ip route add 133.133.133.1 via 12.12.12.1
#sudo ip route add 133.133.133.2 via 12.12.12.1

# Change default route in llb1
$hexec llb1 ip route del default 
$hexec llb1 ip route add default via 12.12.12.254

# Change default route in llb2
$hexec llb2 ip route del default 
$hexec llb2 ip route add default via 14.14.14.254

# Backup paths in user
$hexec user ip route add 124.124.124.1/32 via 2.2.2.254
$hexec user ip route add 125.125.125.1/32 via 2.2.2.254
$hexec user ip route add 134.134.134.1/32 via 2.2.2.254
$hexec user ip route add 135.135.135.1/32 via 2.2.2.254

sleep 1
##Create LB rule
create_lb_rule llb1 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=fullnat --bgp
create_lb_rule llb2 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=fullnat --bgp

# keepalive will take few seconds to be UP and running with valid states
sleep 60

# K3s setup

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# If k3s setup exists, skip installation
if [[ -f "/usr/local/bin/k3s-uninstall.sh" ]]; then
  echo "K3s exists"
  sleep 10
else
  echo "Start K3s installation"

  # Install k3s without external cloud-manager and disabled servicelb
  curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.22.9+k3s1 INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external" K3S_KUBECONFIG_MODE="644" sh -
  #curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external" K3S_KUBECONFIG_MODE="644" sh -

  sleep 10

  # Check kubectl works
  kubectl $KUBECONFIG get pods -A

  # Remove taints in k3s if any (usually happens if started without cloud-manager)
  kubectl $KUBECONFIG taint nodes --all node.cloudprovider.kubernetes.io/uninitialized=false:NoSchedule-

  # Start loxi-ccm as k3s daemonset
  kubectl $KUBECONFIG apply -f https://github.com/loxilb-io/loxi-ccm/raw/master/manifests/loxi-ccm-k3s.yaml

  echo "End K3s installation"
fi

# Install Bird to work with k3s
sudo apt-get install bird2 --yes

sleep 5

sudo cp -f bird_config/bird.conf /etc/bird/bird.conf
if [ ! -f  /var/log/bird.log ]; then
  sudo touch /var/log/bird.log
fi
sudo chown bird:bird /var/log/bird.log
sudo systemctl restart bird

sleep 10

# Wait for cluster to be ready
wait_cluster_ready_full

# Start nginx pods and services for test(using kube-loxilb)
kubectl $KUBECONFIG apply -f kube-loxilb.yml
sleep 15
kubectl $KUBECONFIG apply -f nginx-svc-lb1.yml

#Build sctp-server image locally
#docker build -t loxilb-io/sctp-server .
#docker save --output sctp-server.tar loxilb-io/sctp-server
#sudo k3s ctr images import ./sctp-server.tar
kubectl $KUBECONFIG apply -f sctp-svc-lb.yml
kubectl $KUBECONFIG apply -f sctp-svc-lb2.yml

sleep 50

# Wait for cluster to be ready
wait_cluster_ready_full

# External LB service must be created by now
echo "kubectl $KUBECONFIG get svc"
echo "****************************"
kubectl $KUBECONFIG get svc
echo "kubectl $KUBECONFIG get pods -A"
echo "****************************"
kubectl $KUBECONFIG get pods -A

echo "llb1: loxicmd get lb -o wide"
echo "****************************"
$dexec llb1 loxicmd get lb -o wide
echo "llb1: loxicmd get ep -o wide"
echo "****************************"
$dexec llb1 loxicmd get ep -o wide
#$dexec llb1 cat /etc/shared/keepalive.state
echo "llb2: loxicmd get lb -o wide"
echo "****************************"
$dexec llb2 loxicmd get lb -o wide
echo "llb2: loxicmd get ep -o wide"
echo "****************************"
$dexec llb2 loxicmd get ep -o wide
#$dexec llb2 cat /etc/shared/keepalive.state
