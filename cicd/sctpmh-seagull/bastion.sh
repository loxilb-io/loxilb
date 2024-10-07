apt-get update
apt-get install -y software-properties-common curl wget lksctp-tools jq
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
echo "blacklist sctp" >>  /etc/modprobe.d/blacklist.conf
echo "install sctp /bin/false" >>  /etc/modprobe.d/blacklist.conf

echo "Rebooting Now!"
reboot
"
