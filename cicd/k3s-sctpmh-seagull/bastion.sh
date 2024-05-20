apt-get update
apt-get install -y software-properties-common curl wget lksctp-tools
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
docker run -u root --cap-add SYS_ADMIN -dit -p 80:80 --name tcp_ep ghcr.io/loxilb-io/nginx:stable
docker run -u root --cap-add SYS_ADMIN -dit --entrypoint sctp_darn --name sctp_ep loxilbio/sctp-darn:latest -H 0.0.0.0 -P 9999 -l

echo "blacklist sctp" >>  /etc/modprobe.d/blacklist.conf
echo "install sctp /bin/false" >>  /etc/modprobe.d/blacklist.conf

sysctl -w net.ipv4.conf.eth1.arp_accept=1 >> /etc/sysctl.conf
sysctl -w net.ipv4.conf.eth2.arp_accept=1 >> /etc/sysctl.conf

reboot
"
