export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --disable servicelb --node-ip=${MASTER_IP}"  sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /etc/rancher/k3s/k3s.yaml
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
sudo kubectl apply -f /vagrant/ingress/loxilb-secret.yml
sudo kubectl apply -f /vagrant/ingress/loxilb-ingress-deploy.yml
sudo kubectl apply -f /vagrant/ingress/loxilb-ingress-svc.yml
sudo kubectl apply -f /vagrant/ingress/loxilb-ingress.yml
sleep 30
/vagrant/wait_ready.sh
