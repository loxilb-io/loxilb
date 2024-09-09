export WORKER_ADDR=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
export MASTER_ADDR=$(cat /vagrant/master-ip)
export NODE_TOKEN=$(cat /vagrant/node-token)

sudo mkdir -p /etc/rancher/k3s
sudo cp -f /vagrant/k3s.yaml /etc/rancher/k3s/k3s.yaml

curl -sfL https://get.k3s.io | K3S_URL="https://${MASTER_ADDR}:6443" K3S_TOKEN="${NODE_TOKEN}" INSTALL_K3S_EXEC="--node-ip=${WORKER_ADDR} --node-external-ip=${WORKER_ADDR} --kube-proxy-arg proxy-mode=ipvs --flannel-iface=eth1" sh -
#sudo kubectl apply -f /vagrant/nginx.yml
#sudo kubectl apply -f /vagrant/udp.yml
#sudo kubectl apply -f /vagrant/iperf-service.yml
#sudo kubectl apply -f /vagrant/loxilb.yml
/vagrant/wait_ready.sh
