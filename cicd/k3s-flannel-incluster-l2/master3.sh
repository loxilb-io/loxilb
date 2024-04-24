sudo su
export WORKER_ADDR=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
export MASTER_ADDR=$(cat /vagrant/master-ip)
export NODE_TOKEN=$(cat /vagrant/node-token)
sudo mkdir -p /etc/loxilb
sudo cp /vagrant/lbconfig.txt /etc/loxilb/
sudo cp /vagrant/EPconfig.txt /etc/loxilb/
#curl -fL https://get.k3s.io | K3S_TOKEN=${NODE_TOKEN} sh -s - server --server https://192.168.80.10:6443 --disable traefik --disable servicelb --node-ip=192.168.80.11 external-hostname=192.168.80.11 --node-external-ip=192.168.80.11 --disable-cloud-controller -t ${NODE_TOKEN}
curl -fL https://get.k3s.io | K3S_TOKEN=${NODE_TOKEN} sh -s - server --server https://192.168.80.10:6443 --disable traefik --disable servicelb --node-ip=192.168.80.12 external-hostname=192.168.80.12 --node-external-ip=192.168.80.80 -t ${NODE_TOKEN} --flannel-iface=eth1
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
sudo kubectl apply -f /vagrant/loxilb.yml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
/vagrant/wait_ready.sh
