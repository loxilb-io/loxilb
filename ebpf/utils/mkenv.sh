#!/bin/bash

docker=$1
NSADD="sudo ip netns add "
LBNSCMD="sudo ip netns exec loxilb "
NSCMD="sudo ip netns exec "

if [ $# -eq 0 ]; then
  echo "No arguments supplied"
  docker="none"
fi

if [[ $docker == "docker" ]]; then
  id=`docker ps -f name=loxilb | cut  -d " "  -f 1 | grep -iv  "CONTAINER"`
  echo $id
  pid=`docker inspect -f '{{.State.Pid}}' $id`
  if [ ! -f /var/run/netns/loxilb ]; then
    sudo touch /var/run/netns/loxilb
    sudo mount -o bind /proc/$pid/ns/net /var/run/netns/loxilb
  fi
else
  $NSADD loxilb
fi

$NSADD l3h1
$NSADD l3h2
$NSADD l2h1
$NSADD l2h2
$NSADD l2vxh1
$NSADD l2vxh2
$NSADD l3vxh1
$NSADD l3vxh2
$NSADD l3h3

$LBNSCMD sysctl net.ipv6.conf.all.disable_ipv6=1
$LBNSCMD ifconfig lo up

sudo ip -n loxilb link add enp1 type veth peer name eth0 netns l3h1
sudo ip -n loxilb link set enp1 mtu 9000 up
sudo ip -n l3h1 link set eth0 mtu 7000 up
$LBNSCMD ifconfig enp1 31.31.31.254/24 up
$NSCMD l3h1 ifconfig eth0 31.31.31.1/24 up
$NSCMD l3h1 ip route add default via 31.31.31.254

sudo ip -n loxilb link add enp2 type veth peer name eth0 netns l3h2
sudo ip -n loxilb link set enp2 mtu 9000 up
sudo ip -n l3h2 link set eth0 mtu 7000 up
$LBNSCMD ifconfig enp2 32.32.32.254/24 up
$NSCMD l3h2 ifconfig eth0 32.32.32.1/24 up
$NSCMD l3h2 ip route add default via 32.32.32.254

sudo ip -n loxilb link add enp2v15 type veth peer name eth0 netns l3h3
sudo ip -n loxilb link set enp2v15 mtu 9000 up
sudo ip -n l3h3 link set eth0 mtu 7000 up
$LBNSCMD ifconfig enp2v15 33.33.33.254/24 up
$NSCMD l3h3 ifconfig eth0 33.33.33.1/24 up
$NSCMD l3h3 ip route add default via 33.33.33.254

sudo ip -n loxilb link add enp3 type veth peer name eth0 netns l2h1
sudo ip -n loxilb link set enp3 mtu 9000 up
sudo ip -n l2h1 link set eth0 mtu 7000 up
$NSCMD l2h1 ip link add link eth0 name eth0.100 type vlan id 100
$NSCMD l2h1 ifconfig eth0.100 100.100.100.1/24 up
$NSCMD l2h1 ip route add default via 100.100.100.254

sudo ip -n loxilb link add enp4 type veth peer name eth0 netns l2h2
sudo ip -n loxilb link set enp4 mtu 9000 up
sudo ip -n l2h2 link set eth0 mtu 7000 up
$NSCMD l2h2 ip link add link eth0 name eth0.100 type vlan id 100
$NSCMD l2h2 ifconfig eth0.100 100.100.100.2/24 up
$NSCMD l2h2 ip route add default via 100.100.100.254
$NSCMD l2h2 ip addr add 100.100.100.3/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.4/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.5/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.6/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.7/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.8/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.9/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.10/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.11/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.12/24 dev eth0.100
$NSCMD l2h2 ip addr add 100.100.100.13/24 dev eth0.100

$LBNSCMD brctl addbr vlan100
$LBNSCMD ip link add link enp3 name enp3.100 type vlan id 100
$LBNSCMD ip link add link enp4 name enp4.100 type vlan id 100
$LBNSCMD brctl addif vlan100 enp3.100
$LBNSCMD brctl addif vlan100 enp4.100
$LBNSCMD ifconfig enp3.100 up
$LBNSCMD ifconfig enp4.100 up
$LBNSCMD ifconfig vlan100 100.100.100.254/24 up

sudo ip -n loxilb link add enp5 type veth peer name eth0 netns l2vxh1
sudo ip -n loxilb link set enp5 mtu 9000 up
sudo ip -n l2vxh1 link set eth0 mtu 7000 up
$NSCMD l2vxh1 ifconfig eth0 50.50.50.1/24 up

sudo ip -n loxilb link add enp6 type veth peer name eth0 netns l2vxh2
sudo ip -n loxilb link set enp6 mtu 9000 up
sudo ip -n l2vxh2 link set eth0 mtu 7000 up
$NSCMD l2vxh2 ifconfig eth0 2.2.2.2/24 up
$NSCMD l2vxh2 ip link add vxlan50 type vxlan id 50 local 2.2.2.2 dev eth0 dstport 4789
$NSCMD l2vxh2 ifconfig vxlan50 50.50.50.2/24 up
$NSCMD l2vxh2 bridge fdb append 00:00:00:00:00:00 dst 2.2.2.1 dev vxlan50

$LBNSCMD brctl addbr vlan20
$LBNSCMD brctl addif vlan20 enp6
$LBNSCMD ip link set vlan20 up
$LBNSCMD ip addr add 2.2.2.1/24 dev vlan20
$LBNSCMD ip link add vxlan50 type vxlan id 50 local 2.2.2.1 dev vlan20 dstport 4789
$LBNSCMD ip link set vxlan50 up
$LBNSCMD bridge fdb append 00:00:00:00:00:00 dst 2.2.2.2 dev vxlan50
$LBNSCMD brctl addbr vlan50
$LBNSCMD brctl addif vlan50 vxlan50
$LBNSCMD brctl addif vlan50 enp5
$LBNSCMD ip link set vlan50 up

# Setup l2vxh1
$NSCMD l2vxh1 ip link add link eth0 name eth0.51 type vlan id 51
$NSCMD l2vxh1 ifconfig eth0.51 51.51.51.1/24 up

# Setup l2vxh2
$NSCMD l2vxh2 ip link add link eth0 name eth0.30 type vlan id 30
$NSCMD l2vxh2 ifconfig eth0.30 3.3.3.2/24 up
$NSCMD l2vxh2 ip link add vxlan51 type vxlan id 51 local 3.3.3.2 dev eth0.30 dstport 4789
$NSCMD l2vxh2 ifconfig vxlan51 51.51.51.2/24 up
$NSCMD l2vxh2  bridge fdb append 00:00:00:00:00:00 dst 3.3.3.1 dev vxlan51

# Setup loxilb vxlan51
$LBNSCMD brctl addbr vlan30
$LBNSCMD ip link add link enp6 name enp6.30 type vlan id 30
$LBNSCMD ip link set enp6.30 up
$LBNSCMD brctl addif vlan30 enp6.30
$LBNSCMD ip link set vlan30 up
$LBNSCMD ip addr add 3.3.3.1/24 dev vlan30
$LBNSCMD ip link add vxlan51 type vxlan id 51 local 3.3.3.1 dev vlan30 dstport 4789
$LBNSCMD ip link set vxlan51 up
$LBNSCMD bridge fdb append 00:00:00:00:00:00 dst 3.3.3.2 dev vxlan51
$LBNSCMD brctl addbr vlan51
$LBNSCMD ip link add link enp5 name enp5.51 type vlan id 51
$LBNSCMD ip link set enp5.51 up
$LBNSCMD brctl addif vlan51 vxlan51
$LBNSCMD brctl addif vlan51 enp5.51
$LBNSCMD ip link set vlan51 up
$LBNSCMD bridge fdb add to 06:02:02:03:04:06 dst 3.3.3.2 dev vxlan51
$LBNSCMD bridge fdb add to 06:02:02:03:04:06 dst 3.3.3.2 dev vxlan51 master

#Setup l3vxh1
sudo ip -n loxilb link add enp7 type veth peer name eth0 netns l3vxh1
sudo ip -n loxilb link set enp7 mtu 9000 up
sudo ip -n l3vxh1 link set eth0 mtu 7000 up
$NSCMD l3vxh1 ifconfig eth0 17.17.17.1/24 up
$NSCMD l3vxh1 ip route add default via 17.17.17.254

#Set loxilb
$LBNSCMD ifconfig enp7 17.17.17.254/24 up
sudo ip -n loxilb link add enp8 type veth peer name eth0 netns l3vxh2
sudo ip -n loxilb link set enp8 mtu 9000 up
sudo ip -n l3vxh2 link set eth0 mtu 7000 up
sudo ip -n loxilb link add enp9 type veth peer name eth1 netns l3vxh2
sudo ip -n loxilb link set enp9 mtu 9000 up
sudo ip -n l3vxh2 link set eth1 mtu 7000 up
sudo ip -n loxilb link add enp10 type veth peer name eth2 netns l3vxh2
sudo ip -n loxilb link set enp10 mtu 9000 up
sudo ip -n l3vxh2 link set eth1 mtu 7000 up

$LBNSCMD ip link add bond1 type bond
$LBNSCMD ip link set bond1 type bond mode 802.3ad

$LBNSCMD ip link set enp9 down
$LBNSCMD ip link set enp10 down
$LBNSCMD ip link set enp9 master bond1
$LBNSCMD ip link set enp10 master bond1
$LBNSCMD ip link set enp9 mtu 9000 up
$LBNSCMD ip link set enp10 mtu 9000 up
$LBNSCMD ip link set bond1 mtu 9000 up

$LBNSCMD brctl addbr vlan8
#$LBNSCMD brctl addif vlan8 enp8
$LBNSCMD brctl addif vlan8 bond1
$LBNSCMD ip link set vlan8 up
$LBNSCMD ip addr add 8.8.8.254/24 dev vlan8
$LBNSCMD ip link add vxlan78 type vxlan id 78 local 8.8.8.254 dev vlan8 dstport 4789
$LBNSCMD ip link set vxlan78 up
$LBNSCMD ifconfig vxlan78 78.78.78.254/24 up
$LBNSCMD bridge fdb append 00:00:00:00:00:00 dst 8.8.8.1 dev vxlan78

#Setup l3vxh2
$NSCMD l3vxh2 ip link add bond1 type bond
$NSCMD l3vxh2 ip link set bond1 type bond mode 802.3ad
$NSCMD l3vxh2 ip link set eth1 down
$NSCMD l3vxh2 ip link set eth2 down
$NSCMD l3vxh2 ip link set eth1 master bond1
$NSCMD l3vxh2 ip link set eth2 master bond1
$NSCMD l3vxh2 ip link set eth1 up
$NSCMD l3vxh2 ip link set eth2 up
$NSCMD l3vxh2 ip link set bond1 up
#$NSCMD l3vxh2 ifconfig eth0 8.8.8.1/24 up
$NSCMD l3vxh2 ifconfig bond1 8.8.8.1/24 up
$NSCMD l3vxh2 ip link add vxlan78 type vxlan id 78 local 8.8.8.1 dev bond1 dstport 4789
$NSCMD l3vxh2 ifconfig vxlan78 78.78.78.1/24 up
$NSCMD l3vxh2 ip addr add 18.18.18.1/24 dev vxlan78
$NSCMD l3vxh2  bridge fdb append 00:00:00:00:00:00 dst 8.8.8.254 dev vxlan78
$NSCMD l3vxh2 ip route add default via 78.78.78.254
$LBNSCMD ip route add 18.18.18.0/24 via 78.78.78.1

sudo mkdir -p /opt/netlox/loxilb/
sudo mount -t bpf bpf /opt/netlox/loxilb/
