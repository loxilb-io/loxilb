export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

ip addr add 2001:cafe:43::2/112 dev eth1
ip addr add 3ffe:cafe::9/64 dev eth2

apt-get update
apt-get install -y software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log --net=host --entrypoint /root/loxilb-io/loxilb/loxilb --name loxilb ghcr.io/loxilb-io/loxilb:latest

#sleep 20
#curl -X 'POST' \
#  'http://127.0.0.1:11111/netlox/v1/config/cistate' \
#  -H 'accept: application/json' \
#  -H 'Content-Type: application/json' \
#  -d '{
#  "instance": "default",
#  "state": "MASTER",
#  "vip": "0.0.0.0"
#}'

echo $LOXILB_IP > /vagrant/loxilb-ip
