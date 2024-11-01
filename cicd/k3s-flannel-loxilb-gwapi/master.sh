export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --disable servicelb --node-ip=${MASTER_IP}"  sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /etc/rancher/k3s/k3s.yaml
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/experimental-install.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yaml
sudo kubectl apply -f /vagrant/ingress/loxilb-secret.yml
sudo kubectl apply -f /vagrant/ingress/loxilb-ingress-deploy.yml
sudo kubectl apply -f /vagrant/gatewayclass.yaml
sudo kubectl apply -f /vagrant/gateway.yaml
sudo kubectl apply -f /vagrant/tcpRoute.yaml
sudo kubectl apply -f /vagrant/udpRoute.yaml
sudo kubectl apply -f /vagrant/httpRoute.yaml
sudo kubectl apply -f /vagrant/httpsRoute.yaml
sleep 30
/vagrant/wait_ready.sh
