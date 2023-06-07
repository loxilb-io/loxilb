export WORKER_ADDR=$(ip a |grep global | grep -v '10.0.2.15' | awk '{print $2}' | cut -f1 -d '/')
export MASTER_ADDR=$(cat /vagrant/master-ip)
export NODE_TOKEN=$(cat /vagrant/node-token)

curl -sfL https://get.k3s.io | K3S_URL="https://${MASTER_ADDR}:6443" K3S_TOKEN="${NODE_TOKEN}" INSTALL_K3S_EXEC="--node-ip=${WORKER_ADDR} --node-external-ip=${WORKER_ADDR}" sh -
sudo kubectl apply -f /vagrant/nginx.yml
