sudo su
export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.90' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
curl -fL https://get.k3s.io | sh -s - server --node-ip=192.168.80.10 --disable servicelb --disable traefik --cluster-init --node-external-ip=192.168.80.10 --disable-cloud-controller  --flannel-backend=none --disable-network-policy --cluster-cidr=10.42.0.0/16
sleep 60
echo $MASTER_IP > /vagrant/master-ip
cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /etc/rancher/k3s/k3s.yaml
cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo kubectl apply -f /vagrant/loxilb.yml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
sudo kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.0/manifests/tigera-operator.yaml
# the operator's CRDs must be registered before its custom resources;
# applying a local file is fast enough to lose that race
sudo kubectl wait --for condition=established --timeout=180s \
  crd/installations.operator.tigera.io crd/apiservers.operator.tigera.io
sudo kubectl create -f /vagrant/calico-resources.yaml
/vagrant/wait_ready.sh
