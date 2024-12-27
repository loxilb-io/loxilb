# Setup the bastion host
sudo apt-get update
sudo apt-get -y install socat lksctp-tools
sudo ip link add link eth2 name eth2.5 type vlan id 5
sudo ip addr add 123.123.123.206/24 dev eth2.5
sudo ip link set eth2.5 up

echo "Host is up"
