export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.90' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

MYSECRET=loxilb
curl -fL https://get.k3s.io | K3S_TOKEN=${MYSECRET}  sh -s - server --node-ip=192.168.80.10 --disable servicelb --disable traefik --cluster-init external-hostname=192.168.80.10 --node-external-ip=192.168.80.10 --disable-cloud-controller

sleep 60
service k3s stop
sleep 10
nohup /usr/local/bin/k3s server --node-ip=192.168.80.10 --disable servicelb --disable traefik --cluster-init external-hostname=192.168.80.10 --node-external-ip=192.168.80.10 --disable-cloud-controller 2>&1 >> /dev/null &
sleep 60

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /vagrant/k3s.yaml
#sudo kubectl apply -f /vagrant/loxilb.yml
#sudo kubectl apply -f /vagrant/kube-loxilb.yml
/vagrant/wait_ready.sh
