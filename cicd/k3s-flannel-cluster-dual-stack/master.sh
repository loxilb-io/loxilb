export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
ip addr add 2001:cafe:43::4/112 dev eth1
ip -6 route add default via 2001:cafe:43::2
# The NAT adapter advertises an IPv6 RA default (metric 100) that beats the
# static default above (metric 1024), so replies to the client subnet would
# leave via eth0 and be dropped. Pin the client subnet to loxilb explicitly.
ip -6 route add 3ffe:cafe::/64 via 2001:cafe:43::2 dev eth1
echo '2001:cafe:43::4 master master' | sudo tee -a /etc/hosts
#echo '192.168.80.10 master master' | sudo tee -a /etc/hosts

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik --disable servicelb --disable-cloud-controller --cluster-cidr=2001:cafe:42::/56,192.169.0.0/16 --service-cidr=2001:cafe:43::/112,172.16.0.0/24 --disable-network-policy --node-ip=2001:cafe:43::4,192.168.80.10 --node-external-ip=2001:cafe:43::4,192.168.80.10 --flannel-ipv6-masq" sh -

echo $MASTER_IP > /vagrant/master-ip
sudo cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
sudo cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sudo sed -i -e "s/127.0.0.1/${MASTER_IP}/g" /vagrant/k3s.yaml
sudo kubectl apply -f /vagrant/kube-loxilb.yml
sleep 60
sudo kubectl apply -f /vagrant/nginx6.yml
/vagrant/wait_ready.sh

# loxilb forwards from eBPF and cannot resolve neighbours itself, so it drops
# traffic for this node until its kernel neighbour table holds our MAC. Nothing
# else on this link generates IPv6 traffic, so prime both caches here, as late
# as possible so the entry is still present when validation runs.
ping6 -c2 -W2 2001:cafe:43::2 || true
