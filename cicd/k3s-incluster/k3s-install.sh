source /vagrant/common.sh
source /vagrant/k3s_common.sh

export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

apt-get update
apt-get install -y software-properties-common ethtool

echo "Start K3s installation"

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --kube-proxy-arg metrics-bind-address=0.0.0.0 --kubelet-arg cloud-provider=external" K3S_KUBECONFIG_MODE="644" sh -

sleep 10

# Check kubectl works
kubectl $KUBECONFIG get pods -A

# Remove taints in k3s if any (usually happens if started without cloud-manager)
kubectl $KUBECONFIG taint nodes --all node.cloudprovider.kubernetes.io/uninitialized=false:NoSchedule-

echo "End K3s installation"
sleep 60
wait_cluster_ready_full

kubectl apply -f /vagrant/loxilb.yml
sleep 60
wait_cluster_ready_full

kubectl apply -f /vagrant/kube-loxilb.yml
sleep 30
wait_cluster_ready_full

kubectl apply -f /vagrant/tcp-svc-lb.yml

# Wait for cluster to be ready
wait_cluster_ready_full
