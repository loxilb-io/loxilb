export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --disable servicelb --disable-cloud-controller --kube-proxy-arg proxy-mode=ipvs --flannel-iface=eth1 \
--disable-network-policy \
--node-ip=${MASTER_IP} --node-external-ip=${MASTER_IP} \
--bind-address=${MASTER_IP}" sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /vagrant/k3s.yaml
#sudo kubectl apply -f "https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml"
#sudo kubectl apply -f /vagrant/rmq/rabbitmq.yaml
/vagrant/wait_ready.sh
