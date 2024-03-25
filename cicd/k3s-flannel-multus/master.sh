export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.22.9+k3s1 INSTALL_K3S_EXEC="--disable traefik --disable servicelb --disable-cloud-controller  \
--node-ip=${MASTER_IP} --node-external-ip=${MASTER_IP} \
--bind-address=${MASTER_IP}" sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /vagrant/k3s.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
sudo kubectl apply -f /vagrant/multus/multus-daemonset.yml
sudo kubectl apply -f /vagrant/multus/macvlan.yml
/vagrant/wait_ready.sh

sudo apt update
sudo apt install -y snapd
sudo snap install go --classic

git clone https://github.com/containernetworking/plugins.git
cd plugins
./build_linux.sh
ls bin/macvlan
sudo cp -f ./bin/macvlan /var/lib/rancher/k3s/data/current/bin/
sudo ifconfig eth2 promisc

sudo kubectl apply -f /vagrant/multus/multus-pod.yml
sudo kubectl apply -f /vagrant/multus/multus-service.yml
sudo kubectl apply -f /vagrant/multus/multus-sctp-pod.yml
sudo kubectl apply -f /vagrant/multus/multus-sctp-service.yml
/vagrant/wait_ready.sh
