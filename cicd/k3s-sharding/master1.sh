sudo su
export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.90' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
curl -fL https://get.k3s.io | sh -s - server --node-ip=192.168.80.10 --disable servicelb --disable traefik --cluster-init external-hostname=192.168.80.10 --node-external-ip=192.168.80.80 --disable-cloud-controller --flannel-iface=eth1
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
sleep 60
echo $MASTER_IP > /vagrant/master-ip
cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sed -i -e "s/127.0.0.1/192.168.80.80/g" /vagrant/k3s.yaml
sudo mkdir -p /etc/loxilb
sudo cp /vagrant/lbconfig.txt /etc/loxilb/
sudo cp /vagrant/EPconfig.txt /etc/loxilb/
/vagrant/wait_ready.sh
