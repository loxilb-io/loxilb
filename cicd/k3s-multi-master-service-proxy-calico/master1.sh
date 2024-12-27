sudo su
# Bootstrap the VIP for master-plane LB
ip addr add 192.168.80.80/32 dev lo
apt-get update && apt-get install ipvsadm ipset -y
export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.90' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
mkdir -p /etc/rancher/k3s/
#If you need to login to docker registry
#cp -f /vagrant/registries.yaml /etc/rancher/k3s/registries.yaml
curl -fL https://get.k3s.io | sh -s - server --node-ip=192.168.80.10 --disable servicelb --disable traefik --cluster-init --disable-cloud-controller --flannel-backend=none --kube-proxy-arg proxy-mode=ipvs --disable-network-policy --kube-apiserver-arg=kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP --cluster-cidr=10.42.0.0/16  --node-name master1 --tls-san 192.168.80.80 --node-external-ip=192.168.80.10
kubectl taint nodes 192.168.80.10 node.cloudprovider.kubernetes.io/uninitialized:NoSchedule-
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
sudo kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/tigera-operator.yaml
sudo kubectl create -f /vagrant/custom-resources.yaml
/vagrant/wait_ready.sh
sleep 60
sudo kubectl apply -f /vagrant/custom-resources.yaml
sleep 60
echo $MASTER_IP > /vagrant/master-ip
cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sed -i -e "s/127.0.0.1/192.168.80.80/g" /etc/rancher/k3s/k3s.yaml
cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo mkdir -p /etc/loxilb
sudo cp /vagrant/lbconfig.txt /etc/loxilb/
sudo cp /vagrant/EPconfig.txt /etc/loxilb/
sudo kubectl apply -f /vagrant/loxilb.yml
/vagrant/wait_ready.sh
sysctl net.core.netdev_max_backlog=10000
