export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

apt-get update
apt-get install -y software-properties-common
apt-get install -y iputils-ping curl vim iptables strongswan strongswan-swanctl
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log --net=host --name loxilb ghcr.io/loxilb-io/loxilb:latest -b --cluster=192.168.80.252 --self=1
docker cp loxilb:/usr/local/sbin/loxicmd ./
#docker exec -dt loxilb /root/loxilb-io/loxilb/loxilb -b --cluster=192.168.80.252 --self=1

dexec="docker exec -dt"
$dexec loxilb ip link add vti101 type vti key 101 remote 192.168.90.9 local 192.168.90.253
$dexec loxilb ip link set vti101 up
$dexec loxilb ip addr add 77.77.101.254/24 remote 77.77.101.1/24 dev vti101
$dexec loxilb sysctl -w "net.ipv4.conf.vti101.disable_policy=1"


sudo cp /vagrant/llb2_ipsec_config/ipsec.conf /etc/
sudo cp /vagrant/llb2_ipsec_config/ipsec.secrets /etc/
sudo cp /vagrant/llb2_ipsec_config/charon.conf /etc/strongswan.d/
sudo systemctl restart strongswan-starter


