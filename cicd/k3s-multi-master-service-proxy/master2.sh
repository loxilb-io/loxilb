sudo su
apt-get update && apt-get install ipvsadm ipset -y
export WORKER_ADDR=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
export MASTER_ADDR=$(cat /vagrant/master-ip)
export NODE_TOKEN=$(cat /vagrant/node-token)
sudo mkdir -p /etc/loxilb
sudo cp /vagrant/lbconfig.txt /etc/loxilb/
sudo cp /vagrant/EPconfig.txt /etc/loxilb/
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
curl -fL https://get.k3s.io | K3S_TOKEN=${NODE_TOKEN} sh -s - server --server https://192.168.80.10:6443 --disable traefik --disable servicelb --node-ip=192.168.80.11 --node-external-ip=192.168.80.80 --disable-cloud-controller -t ${NODE_TOKEN} --flannel-iface=eth2 --kube-proxy-arg proxy-mode=ipvs --disable-network-policy --kube-apiserver-arg=kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname --node-name master2 --tls-san 192.168.80.80 --node-external-ip=192.168.80.11
sed -i -e "s/127.0.0.1/192.168.80.80/g" /etc/rancher/k3s/k3s.yaml
kubectl taint nodes master2  node.cloudprovider.kubernetes.io/uninitialized:NoSchedule-
/vagrant/wait_ready.sh
sysctl net.core.netdev_max_backlog=10000
