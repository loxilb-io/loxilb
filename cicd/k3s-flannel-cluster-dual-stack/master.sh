export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
ip addr add 2001:cafe:43::4/112 dev eth1
ip -6 route add default via 2001:cafe:43::2
echo '2001:cafe:43::4 master master' | sudo tee -a /etc/hosts
#echo '192.168.80.10 master master' | sudo tee -a /etc/hosts

curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="v1.25.16+k3s4" INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --cluster-cidr=2001:cafe:42::/56,192.169.0.0/16 --service-cidr=2001:cafe:43::/112,172.16.0.0/24 --disable-network-policy --node-ip=2001:cafe:43::4,192.168.80.10 --node-external-ip=2001:cafe:43::4,192.168.80.10 --flannel-ipv6-masq" sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /vagrant/k3s.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
sleep 60
sudo kubectl apply -f /vagrant/nginx6.yml
/vagrant/wait_ready.sh
