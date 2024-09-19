#!/bin/bash
#
# Setup for Control Plane (Master) servers

set -euxo pipefail

NODENAME=$(hostname -s)

sudo sed -i 's#10.85.0.0/16#10.244.0.0/24#g' /etc/cni/net.d/100-crio-bridge.conflist

sudo kubeadm config images pull

echo "Preflight Check Passed: Downloaded All Required Images"

#sudo kubeadm init --apiserver-advertise-address=$CONTROL_IP --apiserver-cert-extra-sans=$CONTROL_IP --pod-network-cidr=$POD_CIDR --service-cidr=$SERVICE_CIDR --node-name "$NODENAME" --ignore-preflight-errors Swap
sudo kubeadm init --ignore-preflight-errors Swap --config /vagrant/yaml/kubeadm-config.yaml

mkdir -p "$HOME"/.kube
sudo cp -i /etc/kubernetes/admin.conf "$HOME"/.kube/config
sudo chown "$(id -u)":"$(id -g)" "$HOME"/.kube/config

# Save Configs to shared /Vagrant location

# For Vagrant re-runs, check if there is existing configs in the location and delete it for saving new configuration.

config_path="/vagrant/configs"

if [ -d $config_path ]; then
  rm -f $config_path/*
else
  mkdir -p $config_path
fi

cp -i /etc/kubernetes/admin.conf $config_path/config
touch $config_path/join.sh
chmod +x $config_path/join.sh

kubeadm token create --print-join-command > $config_path/join.sh

sudo -i -u vagrant bash << EOF
whoami
mkdir -p /home/vagrant/.kube
sudo cp -i $config_path/config /home/vagrant/.kube/
sudo chown 1000:1000 /home/vagrant/.kube/config
EOF

# Install Flannel Network Plugin
kubectl apply -f /vagrant/yaml/kube-flannel.yml

# Install loxilb checksum module
#curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -

# Install whereabouts
git clone https://github.com/k8snetworkplumbingwg/whereabouts && cd whereabouts
kubectl apply \
    -f doc/crds/daemonset-install.yaml \
    -f doc/crds/whereabouts.cni.cncf.io_ippools.yaml \
    -f doc/crds/whereabouts.cni.cncf.io_overlappingrangeipreservations.yaml && cd -

# Install multus
kubectl apply -f /vagrant/multus/multus-daemonset.yml

# Wait for pods to be ready
kubectl wait pod --all --for=condition=Ready --namespace=kube-system --timeout=240s >> /dev/null 2>&1 || true
kubectl wait pod --all --for=condition=Ready --namespace=default --timeout=240s >> /dev/null 2>&1 || true
kubectl wait pod --all --for=condition=Ready --namespace=kube-flannel --timeout=240s  >> /dev/null 2>&1 || true
kubectl apply -f /vagrant/multus/multus-vlan.yml
sleep 60
kubectl apply -f /vagrant/yaml/loxilb.yaml
