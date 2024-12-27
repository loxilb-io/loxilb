ip addr add 3ffe:cafe::8/64 dev eth2

sysctl net.ipv6.conf.all.proxy_ndp=1
sysctl net.ipv6.conf.default.proxy_ndp=1 
sysctl net.ipv6.conf.eth2.proxy_ndp=1 

apt-get update
apt-get install -y software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
