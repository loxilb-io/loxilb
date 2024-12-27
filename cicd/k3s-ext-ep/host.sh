echo "20.20.20.1 k8s-svc" >> /etc/hosts
apt-get update
apt-get install -y software-properties-common lksctp-tools
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
docker run --cap-add SYS_ADMIN -dit --net=host --name tcp_ep ghcr.io/loxilb-io/nginx:stable
sudo ip route add 20.20.20.1 via 192.168.82.100
echo "Host is up"
