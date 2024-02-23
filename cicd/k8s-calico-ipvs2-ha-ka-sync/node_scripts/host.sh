
sysctl net.ipv4.conf.all.arp_accept=1
sysctl net.ipv4.conf.eth1.arp_accept=1

sudo apt-get update
sudo apt-get -y install lksctp-tools iperf

echo "Host is up"
