#!/bin/bash
#
# Setup for Node servers

set -euxo pipefail

if [[ $(hostname -s) == "worker1" ]]; then
  sudo sed -i 's#10.85.0.0/16#10.244.1.0/24#g' /etc/cni/net.d/100-crio-bridge.conflist
else
  sudo sed -i 's#10.85.0.0/16#10.244.2.0/24#g' /etc/cni/net.d/100-crio-bridge.conflist
fi

config_path="/vagrant/configs"

/bin/bash $config_path/join.sh -v

sudo -i -u vagrant bash << EOF
whoami
mkdir -p /home/vagrant/.kube
sudo cp -i $config_path/config /home/vagrant/.kube/
sudo chown 1000:1000 /home/vagrant/.kube/config
NODENAME=$(hostname -s)
kubectl label node $(hostname -s) node-role.kubernetes.io/worker=worker
kubectl wait pod --all --for=condition=Ready --namespace=kube-system --timeout=240s >> /dev/null 2>&1 || true
kubectl wait pod --all --for=condition=Ready --namespace=default --timeout=240s >> /dev/null 2>&1 || true
kubectl wait pod --all --for=condition=Ready --namespace=kube-flannel --timeout=240s  >> /dev/null 2>&1 || true
kubectl apply -f /vagrant/yaml/kube-loxilb.yaml
kubectl apply -f /vagrant/multus/multus-pod.yml
sleep 60
kubectl apply -f /vagrant/multus/multus-service.yml

EOF

#curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
