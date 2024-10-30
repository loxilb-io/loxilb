sudo apt-get install -y lksctp-tools
sudo ip route add 123.123.123.0/24 via 192.168.90.10
sysctl net.ipv4.conf.eth1.arp_accept=1
sysctl net.ipv4.conf.eth2.arp_accept=1
sysctl net.ipv4.conf.default.arp_accept=1
echo "Host is up"
