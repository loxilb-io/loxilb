sudo su
sudo apt-get install -y lksctp-tools socat
wget https://github.com/loxilb-io/loxilb/raw/main/cicd/common/sctp_client
wget https://github.com/loxilb-io/loxilb/raw/main/cicd/common/udp_client
chmod 777 sctp_client
chmod 777 udp_client
echo "123.123.123.1 k8s-svc" >> /etc/hosts
ifconfig eth2 mtu 1450
ip route add 123.123.123.0/24 via 192.168.90.10
echo "Host is up"
