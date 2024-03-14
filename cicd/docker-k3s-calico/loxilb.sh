source /vagrant/common.sh
source /vagrant/k3s_common.sh

export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

## Set promisc mode for mac-vlan to work
sudo ifconfig eth1 promisc

apt-get update
apt-get install -y software-properties-common ethtool
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get install -y docker-ce

echo "Start loxilb installation"
sudo docker run -u root --cap-add SYS_ADMIN --restart unless-stopped --privileged --entrypoint /root/loxilb-io/loxilb/loxilb -dit -v /dev/log:/dev/log  --name loxilb ghcr.io/loxilb-io/loxilb:latest

# Create mac-vlan on top of underlying eth1 interface
docker network create -d macvlan -o parent=eth1 --subnet 192.168.163.0/24   --gateway 192.168.163.1 --aux-address 'host=192.168.163.252' llbnet

# Assign mac-vlan to loxilb docker with specified IP (which will be used as LB VIP)
docker network connect llbnet loxilb --ip=192.168.163.247

# Add iptables rule to allow traffic from source IP(192.168.163.1) to loxilb
sudo iptables -A DOCKER -s 192.168.163.1 -j ACCEPT

echo "Start K3s installation"

# Install K3s with Calico and IPVS
sudo apt-get -y install ipset ipvsadm
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik,metrics-server,servicelb --disable-cloud-controller --kubelet-arg cloud-provider=external --flannel-backend=none --disable-network-policy" K3S_KUBECONFIG_MODE="644" sh -s - server --kube-proxy-arg proxy-mode=ipvs
sleep 10

# Install Calico
kubectl $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/tigera-operator.yaml

kubectl $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/custom-resources.yaml

# Check kubectl works
kubectl $KUBECONFIG get pods -A

# Remove taints in k3s if any (usually happens if started without cloud-manager)
kubectl $KUBECONFIG taint nodes --all node.cloudprovider.kubernetes.io/uninitialized=false:NoSchedule-

echo "End K3s installation"
sleep 60

kubectl apply -f /vagrant/kube-loxilb.yml
sleep 60
kubectl apply -f /vagrant/tcp-svc-lb.yml

# Wait for cluster to be ready
wait_cluster_ready_full

echo $LOXILB_IP > /vagrant/loxilb-$(hostname)
