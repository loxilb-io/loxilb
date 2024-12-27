# Install Bird to work with k3s
sudo apt-get update
sudo apt-get -y install bird2 lksctp-tools iperf
sudo apt-get install -y iputils-ping curl vim iptables strongswan strongswan-swanctl

sudo ip addr add 30.30.30.1/32 dev lo

sudo ip link add vti100 type vti key 100 remote 192.168.90.252 local 192.168.90.9
sudo ip link set vti100 up
sudo ip addr add 77.77.100.1/24 remote 77.77.100.254/24 dev vti100
sudo sysctl -w "net.ipv4.conf.vti100.disable_policy=1"

sudo ip link add vti101 type vti key 101 remote 192.168.90.253 local 192.168.90.9
sudo ip link set vti101 up
sudo ip addr add 77.77.101.1/24 remote 77.77.101.254/24 dev vti101
sudo sysctl -w "net.ipv4.conf.vti101.disable_policy=1"

sudo cp /vagrant/host_ipsec_config/ipsec.conf /etc/
sudo cp /vagrant/host_ipsec_config/ipsec.secrets /etc/
sudo cp /vagrant/host_ipsec_config/charon.conf /etc/strongswan.d/
sudo systemctl restart strongswan-starter

sudo cp -f /vagrant/bird_config/bird.conf /etc/bird/bird.conf
if [ ! -f  /var/log/bird.log ]; then
  sudo touch /var/log/bird.log
fi
sudo chown bird:bird /var/log/bird.log
sudo service bird restart
echo "Host is up"
