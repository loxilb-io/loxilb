sudo su
export WORKER_ADDR=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
export MASTER_ADDR=$(cat /vagrant/master-ip)
export NODE_TOKEN=$(cat /vagrant/node-token)

#curl -fL https://get.k3s.io | K3S_TOKEN=${NODE_TOKEN} sh -s - server --server https://192.168.80.10:6443 --disable traefik --disable servicelb --node-ip=192.168.80.11 external-hostname=192.168.80.11 --node-external-ip=192.168.80.11 --disable-cloud-controller -t ${NODE_TOKEN}
curl -fL https://get.k3s.io | K3S_TOKEN=${NODE_TOKEN} sh -s - server --server https://192.168.80.10:6443 --disable traefik --disable servicelb --node-ip=192.168.80.11 external-hostname=192.168.80.11 --node-external-ip=192.168.80.11 -t ${NODE_TOKEN}
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -

/vagrant/wait_ready.sh
