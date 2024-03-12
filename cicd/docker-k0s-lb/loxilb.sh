source /vagrant/common.sh

function wait_k0s_cluster_ready {
    Res=$(sudo k0s kubectl get pods -A |
    while IFS= read -r line; do
        if [[ "$line" != *"Running"* && "$line" != *"READY"* ]]; then
            echo "not ready"
            return
        fi
    done)
    if [[ $Res == *"not ready"* ]]; then
        return 1
    fi
    return 0
}

function wait_k0s_cluster_ready_full {
  i=1
  nr=0
  for ((;;)) do
    wait_k0s_cluster_ready
    nr=$?
    if [[ $nr == 0 ]]; then
        echo "Cluster is ready"
        break
    fi
    i=$(( $i + 1 ))
    if [[ $i -ge 40 ]]; then
        echo "Cluster is not ready.Giving up"
        sudo k0s kubectl get svc
        sudo k0s kubectl get pods -A
        exit 1
    fi
    echo "Cluster is not ready...."
    sleep 10
  done
}

export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

## Set promisc mode for mac-vlan to work
sudo ifconfig eth1 promisc

apt-get update
apt-get install -y software-properties-common ethtool ipvsadm ipset -y
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get install -y docker-ce
sudo docker run -u root --cap-add SYS_ADMIN --restart unless-stopped --privileged --entrypoint /root/loxilb-io/loxilb/loxilb -dit -v /dev/log:/dev/log  --name loxilb ghcr.io/loxilb-io/loxilb:latest

#docker exec -i loxilb apt-get update
#docker exec -i loxilb apt-get -y install clang-10 llvm libelf-dev gcc-multilib libpcap-dev linux-tools-$(uname -r) elfutils dwarves git libbsd-dev bridge-utils unzip build-essential bison flex iperf iproute2 nodejs socat ethtool

# Create mac-vlan on top of underlying eth1 interface
docker network create -d macvlan -o parent=eth1 --subnet 192.168.82.0/24   --gateway 192.168.82.1 --aux-address 'host=192.168.82.252' llbnet

# Assign mac-vlan to loxilb docker with specified IP (which will be used as LB VIP)
docker network connect llbnet loxilb --ip=192.168.82.100

# Start a docker to simulate e2 sctp endpoint
docker run -u root --cap-add SYS_ADMIN -dit --privileged --name e2 eyes852/ubuntu-iperf-test:0.5
docker exec -i e2 apt-get update
docker exec -i e2 apt-get -y install lksctp-tools

# Add iptables rule to allow traffic from source IP(192.168.163.1) to loxilb
sudo iptables -A DOCKER -s 192.168.82.1 -j ACCEPT

echo "Start K0s installation"

curl -sSLf https://get.k0s.sh | sudo sh
k0s install controller --single
k0s start

sleep 30
k0s status

# Check kubectl works
k0s kubectl $KUBECONFIG get pods -A

echo "End K0s installation"
sleep 60

k0s kubectl apply -f /vagrant/kube-loxilb.yml
sleep 30
k0s kubectl apply -f /vagrant/tcp-svc-lb.yml

# Wait for cluster to be ready
wait_k0s_cluster_ready_full

echo $LOXILB_IP > /vagrant/loxilb-$(hostname)
