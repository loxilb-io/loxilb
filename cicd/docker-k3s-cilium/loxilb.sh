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

#docker exec -i loxilb apt-get update
#docker exec -i loxilb apt-get -y install clang-10 llvm libelf-dev gcc-multilib libpcap-dev linux-tools-$(uname -r) elfutils dwarves git libbsd-dev bridge-utils unzip build-essential bison flex iperf iproute2 nodejs socat ethtool

# Create mac-vlan on top of underlying eth1 interface
docker network create -d macvlan -o parent=eth1 --subnet 192.168.163.0/24   --gateway 192.168.163.1 --aux-address 'host=192.168.163.252' llbnet

# Assign mac-vlan to loxilb docker with specified IP (which will be used as LB VIP)
docker network connect llbnet loxilb --ip=192.168.163.247

# Start a docker to simulate e2 sctp endpoint
docker run -u root --cap-add SYS_ADMIN -dit --privileged --name e2 eyes852/ubuntu-iperf-test:0.5
docker exec -i e2 apt-get update
docker exec -i e2 apt-get -y install lksctp-tools

# Add iptables rule to allow traffic from source IP(192.168.163.1) to loxilb
sudo iptables -A DOCKER -s 192.168.163.1 -j ACCEPT

# Start application to simulate sctp end-point
#docker exec -i e2 nohup sctp_darn -H 172.17.0.3  -P 5003 -l 2>&1  &

echo "Start K3s installation"

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller  --disable "metrics-server" --kubelet-arg cloud-provider=external --flannel-backend=none --cluster-cidr=10.42.0.0/16" K3S_KUBECONFIG_MODE="644" sh -

sleep 10

# Install Cilium
echo "Start cilium installation"
CILIUM_CLI_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/cilium-cli/master/stable.txt)
CLI_ARCH=amd64
if [ "$(uname -m)" = "aarch64" ]; then CLI_ARCH=arm64; fi
  curl -L --fail --remote-name-all https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum}
sha256sum --check cilium-linux-${CLI_ARCH}.tar.gz.sha256sum
sudo tar xzvfC cilium-linux-${CLI_ARCH}.tar.gz /usr/local/bin
rm cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum}

mkdir ~/.kube/
cat /etc/rancher/k3s/k3s.yaml > ~/.kube/config
cilium install

sleep 60

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
