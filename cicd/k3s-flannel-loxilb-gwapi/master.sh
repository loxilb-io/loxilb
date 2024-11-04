export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

apt-get update && apt install -y libnss3-tools
ldconfig /usr/local/lib64/ | true
mkdir certs
cd certs
wget --retry-connrefused --waitretry=1 --read-timeout=20 --timeout=15 -t 3  https://github.com/FiloSottile/mkcert/releases/download/v1.4.3/mkcert-v1.4.3-linux-amd64
chmod +x mkcert-v1.4.3-linux-amd64
mv mkcert-v1.4.3-linux-amd64 mkcert
mkdir loxilb.io
export CAROOT=`pwd`/loxilb.io
./mkcert -install
./mkcert loxilb.io
mv loxilb.io.pem ../server.crt
mv loxilb.io-key.pem ../server.key
cd -

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --disable servicelb --node-ip=${MASTER_IP}"  sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /etc/rancher/k3s/k3s.yaml
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo kubectl create secret tls loxilb-ssl --cert server.crt --key server.key -n kube-system -o yaml --dry-run >> loxilb-secret.yml
sed -i -e 's/tls.key/server.key/g' ./loxilb-secret.yml
sed -i -e 's/tls.crt/server.crt/g' ./loxilb-secret.yml
sed -i -e 's/kubernetes.io\/tls/Opaque/g' ./loxilb-secret.yml
sudo kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/experimental-install.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yaml
sudo kubectl apply -f loxilb-secret.yml
sudo kubectl apply -f /vagrant/ingress/loxilb-ingress-deploy.yml
sudo kubectl apply -f /vagrant/gatewayclass.yaml
sudo kubectl apply -f /vagrant/gateway.yaml
sudo kubectl apply -f /vagrant/tcpRoute.yaml
sudo kubectl apply -f /vagrant/udpRoute.yaml
sudo kubectl apply -f /vagrant/httpRoute.yaml
sudo kubectl apply -f /vagrant/httpsRoute.yaml
sleep 30
/vagrant/wait_ready.sh
