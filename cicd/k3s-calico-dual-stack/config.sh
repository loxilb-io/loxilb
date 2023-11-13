#!/bin/bash

source ../common.sh
source ../k3s_common.sh

sudo sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
sudo sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
sudo sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type host --dock-name ep1
spawn_docker_host --dock-type host --dock-name ep2
spawn_docker_host --dock-type host --dock-name ep3
spawn_docker_host --dock-type host --dock-name r1
spawn_docker_host --dock-type host --dock-name r2
spawn_docker_host --dock-type host --dock-name user

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts user r1
connect_docker_hosts r1 llb1
connect_docker_hosts llb1 r2
connect_docker_hosts r2 ep1
connect_docker_hosts r2 ep2
connect_docker_hosts r2 ep3

$hexec user sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec user sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec user sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

$hexec r1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec r1 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec r1 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

$hexec llb1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec llb1 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec llb1 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

$hexec r2 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec r2 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec r2 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

$hexec ep1 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec ep1 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec ep1 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

$hexec ep2 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec ep2 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec ep2 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

$hexec ep3 sysctl net.ipv6.conf.all.disable_ipv6=0 2>&1 >> /dev/null
$hexec ep3 sysctl net.ipv6.conf.default.disable_ipv6=0 2>&1 >> /dev/null
$hexec ep3 sysctl net.ipv6.conf.all.forwarding=1 2>&1 >> /dev/null

## Make network for k3s connectivity
sudo ip link add ellb1sys type veth peer name esysllb1
sleep 3
sudo ip link set ellb1sys netns llb1
sleep 3
sudo ip -n llb1 link set ellb1sys up
sudo ip -n llb1 addr add 12.12.12.1/24 dev ellb1sys
$hexec llb1 ip -6 addr add 8ffe::1/96 dev ellb1sys

# Node-IP
sudo ip link set esysllb1 up
sudo ip addr add 12.12.12.254/24 dev esysllb1
sudo ip -6 addr add 8ffe::2/96 dev esysllb1

#node1 config
config_docker_host --host1 user --host2 r1 --ptype phy --addr 1.1.1.1/24 --gw 1.1.1.254
config_docker_host --host1 r1 --host2 user --ptype phy --addr 1.1.1.254/24

config_docker_host --host1 r1 --host2 llb1 --ptype phy --addr 11.11.11.254/24 --gw 11.11.11.1
config_docker_host --host1 llb1 --host2 r1 --ptype phy --addr 11.11.11.1/24

config_docker_host --host1 llb1 --host2 r2 --ptype phy --addr 10.10.10.1/24

config_docker_host --host1 r2 --host2 llb1 --ptype phy --addr 10.10.10.254/24

config_docker_host --host1 r2 --host2 ep1 --ptype phy --addr 31.31.31.254/24

config_docker_host --host1 ep1 --host2 r2 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254

config_docker_host --host1 r2 --host2 ep2 --ptype phy --addr 32.32.32.254/24

config_docker_host --host1 ep2 --host2 r2 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254

config_docker_host --host1 r2 --host2 ep3 --ptype phy --addr 33.33.33.254/24

config_docker_host --host1 ep3 --host2 r2 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254

##Pod networks
$hexec r1 ip route add 20.20.20.1/32 via 11.11.11.1
#add_route llb1 1.1.1.0/24 11.11.11.254

sleep 1
##Create LB rule
create_lb_rule llb1 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=fullnat

## IPV6 Stuff
$hexec user ip -6 addr add 3ffe::1/64 dev euserr1
$hexec user ip -6 route add default via 3ffe::10
$hexec user ethtool --offload  euserr1 rx off  tx off
$hexec user ethtool -K euserr1 gso off

$hexec r1 ip -6 addr add 3ffe::10/64 dev er1user
$hexec r1 ethtool --offload  er1user rx off  tx off
$hexec r1 ethtool -K er1user gso off

$hexec r1 ip -6 addr add 4ffe::10/64 dev er1llb1
$hexec r1 ip -6 route add default via 4ffe::1
$hexec r1 ethtool --offload  er1llb1 rx off  tx off
$hexec r1 ethtool -K er1llb1 gso off

$hexec llb1 ip -6 addr add 4ffe::1/64 dev ellb1r1
$hexec llb1 ethtool --offload ellb1r1 rx off tx off
$hexec llb1 ethtool -K ellb1r1 gso off

$hexec llb1 ip -6 addr add 5ffe::1/64 dev ellb1r2
$hexec llb1 ethtool --offload ellb1r2 rx off tx off
$hexec llb1 ethtool -K ellb1r2 gso off

$hexec r2 ip -6 addr add 5ffe::10/64 dev er2llb1
$hexec r2 ethtool --offload  er2llb1 rx off  tx off
$hexec r2 ethtool -K er2llb1 gso off

#Default route towards r1
$hexec llb1 ip -6 route add default via 5ffe::10

#Default route towards llb1
$hexec r2 ip -6 route add default via 5ffe::1

$hexec r2 ip -6 addr add 6ffa::10/64 dev er2ep1
$hexec r2 ethtool --offload  er2ep1 rx off  tx off
$hexec r2 ethtool -K er2ep1 gso off

$hexec r2 ip -6 addr add 6ffb::10/64 dev er2ep2
$hexec r2 ethtool --offload  er2ep2 rx off  tx off
$hexec r2 ethtool -K er2ep2 gso off

$hexec r2 ip -6 addr add 6ffc::10/64 dev er2ep3
$hexec r2 ethtool --offload  er2ep3 rx off  tx off
$hexec r2 ethtool -K er2ep3 gso off

$hexec ep1 ip -6 addr add 6ffa::1/64 dev eep1r2
$hexec ep1 ip -6 route add default via 6ffa::10
$hexec ep1 ethtool --offload  eep1r2 rx off  tx off
$hexec ep1 ethtool -K eep1r2 gso off


$hexec ep2 ip -6 addr add 6ffb::1/64 dev eep2r2
$hexec ep2 ip -6 route add default via 6ffb::10
$hexec ep2 ethtool --offload  eep2r2 rx off  tx off
$hexec ep2 ethtool -K eep2r2 gso off

$hexec ep3 ip -6 addr add 6ffc::1/64 dev eep3r2
$hexec ep3 ip -6 route add default via 6ffc::10
$hexec ep3 ethtool --offload  eep3r2 rx off  tx off
$hexec ep3 ethtool -K eep3r2 gso off

$hexec llb1 ip addr add 2001::1/128 dev lo

#NAT64 service
$dexec llb1 loxicmd create lb 2001::1 --tcp=1064:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1

#NAT66 service
$dexec llb1 loxicmd create lb 2001::1 --tcp=1066:8080 --endpoints=6ffa::1:1,6ffb::1:1,6ffc::1:1

sleep 2

# K3s setup
KUBECONFIG=--kubeconfig=/etc/rancher/k3s/k3s.yaml
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
  #curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.22.9+k3s1 INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external" K3S_KUBECONFIG_MODE="644" sh -
  curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.22.9+k3s1 INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external --flannel-backend=none --disable-network-policy --cluster-cidr=10.42.0.0/16,4dde::/64 --service-cidr=10.43.0.0/16,5dde::/108 --node-ip=12.12.12.254,8ffe::2" K3S_KUBECONFIG_MODE="644" sh -

  sleep 10

  # Install Calico
  kubectl $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/tigera-operator.yaml

  #kubectl $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/custom-resources.yaml
  kubectl $KUBECONFIG create -f custom-resources.yaml

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

# Start nginx pods and services for test
kubectl $KUBECONFIG apply -f nginx.yml
kubectl $KUBECONFIG apply -f nginx-svc-lb.yml

sleep 5 

# Start nginx pods and services for test(using kube-loxilb)
kubectl $KUBECONFIG apply -f kube-loxilb.yml
sleep 15
kubectl $KUBECONFIG apply -f nginx-svc-lb1.yml
sleep 10
kubectl $KUBECONFIG apply -f udp-svc-lb.yml
sleep 10
kubectl $KUBECONFIG apply -f sctp-svc-lb.yml
sleep 10
kubectl $KUBECONFIG apply -f nginx-svc-lb1-ipv6.yml
sleep 30

# External LB service must be created by now
kubectl $KUBECONFIG get svc

#Route back to llb1
sudo ip route add 1.1.1.1/32 via 12.12.12.1
sudo ip route add 3ffe::1/128 via 8ffe::1

# Route back to user
$hexec llb1 ip route add 1.1.1.0/24 via 11.11.11.254
$hexec llb1 ip -6 route add 3ffe::0/64 via 4ffe::10


# Wait for cluster to be ready
wait_cluster_ready_full
