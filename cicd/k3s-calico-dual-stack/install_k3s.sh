#!/bin/bash

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
  #curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.22.9+k3s1 INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external --flannel-backend=none --cluster-cidr=10.42.0.0/16" K3S_KUBECONFIG_MODE="644" sh -
  curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.22.9+k3s1 INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external --flannel-backend=none --disable-network-policy --cluster-cidr=10.42.0.0/16,4dde::/64 --service-cidr=10.43.0.0/16,5dde::/108 --node-ip=12.12.12.254,8ffe::2" K3S_KUBECONFIG_MODE="644" sh -

  sleep 10

  # Install Calico
  kubectl $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/tigera-operator.yaml

  #kubectl $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/custom-resources.yaml
  kubectl $KUBECONFIG apply -f custom-resources.yaml

  # Check kubectl works
  kubectl $KUBECONFIG get pods -A

  # Remove taints in k3s if any (usually happens if started without cloud-manager)
  kubectl $KUBECONFIG taint nodes --all node.cloudprovider.kubernetes.io/uninitialized=false:NoSchedule-

  # Start loxi-ccm as k3s daemonset
  kubectl $KUBECONFIG apply -f https://github.com/loxilb-io/loxi-ccm/raw/master/manifests/loxi-ccm-k3s.yaml

  echo "End K3s installation"
fi
