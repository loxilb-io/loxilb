sudo su
export WORKER_ADDR=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
export MASTER_ADDR=$(cat /vagrant/master-ip)
export NODE_TOKEN=$(cat /vagrant/node-token)
mkdir -p /etc/rancher/k3s
cat <<EOF > /etc/rancher/k3s/config.yaml
server: https://192.168.80.10:6443
token: ${NODE_TOKEN}
node-ip: 192.168.80.11
node-external-ip: 192.168.80.11
disable:
  - servicelb
  - traefik
disable-cloud-controller: true
flannel-iface: eth1
node-name: master2
tls-san:
  - 192.168.80.80
EOF
sudo mkdir -p /etc/loxilb
sudo cp /vagrant/lbconfig.txt /etc/loxilb/
sudo cp /vagrant/EPconfig.txt /etc/loxilb/
curl -fL https://get.k3s.io | sh -s - server
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
#sudo kubectl apply -f /vagrant/loxilb.yml
#sudo kubectl apply -f /vagrant/kube-loxilb.yml
