export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

apt-get update
apt-get install -y software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get install -y docker-ce
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged --entrypoint /bin/bash -dit -v /dev/log:/dev/log --net=host --name loxilb ghcr.io/loxilb-io/loxilb:latest

docker exec -it loxilb apt-get update
docker exec -it loxilb apt-get -y install clang-10 llvm libelf-dev gcc-multilib libpcap-dev linux-tools-$(uname -r) elfutils dwarves git libbsd-dev bridge-utils unzip build-essential bison flex iperf iproute2 nodejs socat ethtool
docker exec -it loxilb git clone https://github.com/loxilb-io/loxilb.git --recurse-submodules

echo alias loxicmd=\"sudo docker exec -it loxilb loxicmd\" >> ~/.bashrc
echo alias loxilb=\"sudo docker exec -it loxilb \" >> ~/.bashrc

echo $LOXILB_IP > /vagrant/loxilb-$(hostname)
