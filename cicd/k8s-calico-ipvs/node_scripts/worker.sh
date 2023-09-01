#!/bin/bash
#
# Setup for Node servers

set -euxo pipefail

config_path="/vagrant/configs"

/bin/bash $config_path/join.sh -v

sudo -i -u vagrant bash << EOF
whoami
mkdir -p /home/vagrant/.kube
sudo cp -i $config_path/config /home/vagrant/.kube/
sudo chown 1000:1000 /home/vagrant/.kube/config
NODENAME=$(hostname -s)
kubectl label node $(hostname -s) node-role.kubernetes.io/worker=worker
EOF

#Install routes for pod to client (fullnat service) in nodes
sudo ip route add 123.123.123.1 via 192.168.80.9
sudo ip route add 124.124.124.1 via 192.168.80.9
sudo ip route add 125.125.125.1 via 192.168.80.9
sudo ip route add 9.9.9.9 via 192.168.80.9

#Install routes for pod to client (default service) in nodes
sudo ip route add 192.168.90.0/24 via 192.168.80.9
