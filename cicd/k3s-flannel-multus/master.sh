export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --disable servicelb --disable-cloud-controller  \
--node-ip=${MASTER_IP} --node-external-ip=${MASTER_IP} \
--bind-address=${MASTER_IP}" sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /vagrant/k3s.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
sudo kubectl apply -f /vagrant/multus/multus-daemonset.yml
sudo kubectl apply -f /vagrant/multus/macvlan.yml
/vagrant/wait_ready.sh

# k3s only ships a subset of the reference CNI plugins, so macvlan has to be
# installed separately. Take it from an upstream release instead of building it
# from source. Since the Oct-2024 k3s releases the CNI bin dir is a stable path;
# older releases used a per-release hash dir (k3s-io/k3s#10869).
CNI_BIN_DIR=/var/lib/rancher/k3s/data/cni
[ -d "$CNI_BIN_DIR" ] || CNI_BIN_DIR=/var/lib/rancher/k3s/data/current/bin
if [ ! -x "$CNI_BIN_DIR/macvlan" ]; then
  CNI_PLUGINS_VERSION=v1.9.1
  curl -sfL -o /tmp/cni-plugins.tgz \
    "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-amd64-${CNI_PLUGINS_VERSION}.tgz"
  sudo tar -xzf /tmp/cni-plugins.tgz -C "$CNI_BIN_DIR" ./macvlan
fi
ls -l "$CNI_BIN_DIR/macvlan"
sudo ifconfig eth2 promisc

# Wait for multus to publish its CNI config and for the container runtime to
# pick it up. Pods created before that come up with only the default flannel
# interface and no secondary macvlan interface.
MULTUS_CONF=/var/lib/rancher/k3s/agent/etc/cni/net.d/00-multus.conflist
for i in $(seq 1 60); do
  sudo test -f "$MULTUS_CONF" && break
  sleep 2
done
sudo test -f "$MULTUS_CONF" || { echo "multus CNI config was never created"; exit 1; }
sleep 15

sudo kubectl apply -f /vagrant/multus/multus-pod.yml
sudo kubectl apply -f /vagrant/multus/multus-service.yml
sudo kubectl apply -f /vagrant/multus/multus-sctp-pod.yml
sudo kubectl apply -f /vagrant/multus/multus-sctp-service.yml
/vagrant/wait_ready.sh
