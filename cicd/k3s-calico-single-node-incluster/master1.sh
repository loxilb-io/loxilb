sudo su
export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.90' | grep '192.168.82' | awk '{print $2}' | cut -f1 -d '/')
curl -fL https://get.k3s.io | sh -s - server --node-ip=192.168.82.128 --disable servicelb --disable traefik --cluster-init external-hostname=192.168.82.128 --node-external-ip=192.168.82.128 --disable-cloud-controller  --kubelet-arg cloud-provider=external --flannel-backend=none --disable-network-policy --cluster-cidr=172.16.219.0/24
sleep 60
echo $MASTER_IP > /vagrant/master-ip
cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /etc/rancher/k3s/k3s.yaml
cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
#sudo kubectl apply -f /vagrant/loxilb.yml
#sudo kubectl apply -f /vagrant/kube-loxilb.yml
sudo kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/tigera-operator.yaml
sudo kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/custom-resources.yaml
/vagrant/wait_ready.sh
