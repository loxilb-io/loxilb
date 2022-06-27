#!/bin/bash
sudo ip netns add loxilb
sudo ip netns add l3h1
sudo ip netns add l3h2
sudo ip netns add l2h1
sudo ip netns add l2h2
sudo ip netns add l2vxh1
sudo ip netns add l2vxh2
sudo ip netns add l3vxh1
sudo ip netns add l3vxh2
sudo ip netns exec loxilb sysctl net.ipv6.conf.all.disable_ipv6=1
sudo ip netns exec loxilb ifconfig lo up

sudo ip -n loxilb link add hs1 type veth peer name eth0 netns l3h1
sudo ip -n loxilb link set hs1 mtu 9000 up
sudo ip -n l3h1 link set eth0 mtu 7000 up
sudo ip netns exec loxilb ifconfig hs1 31.31.31.254/24 up
sudo ip netns exec l3h1 ifconfig eth0 31.31.31.1/24 up
sudo ip netns exec l3h1 ip route add default via 31.31.31.254
sudo ip netns exec l3h1 ethtool -K eth0 rxvlan off txvlan off

sudo ip -n loxilb link add hs2 type veth peer name eth0 netns l3h2
sudo ip -n loxilb link set hs2 mtu 9000 up
sudo ip -n l3h2 link set eth0 mtu 7000 up
sudo ip netns exec loxilb ifconfig hs2 32.32.32.254/24 up
sudo ip netns exec l3h2 ifconfig eth0 32.32.32.1/24 up
sudo ip netns exec l3h2 ip route add default via 32.32.32.254

sudo ip netns add l3h3
sudo ip -n loxilb link add hs2v15 type veth peer name eth0 netns l3h3
sudo ip -n loxilb link set hs2v15 mtu 9000 up
sudo ip -n l3h3 link set eth0 mtu 7000 up
sudo ip netns exec loxilb ifconfig hs2v15 33.33.33.254/24 up
sudo ip netns exec l3h3 ifconfig eth0 33.33.33.1/24 up
sudo ip netns exec l3h3 ip route add default via 33.33.33.254

sudo ip -n loxilb link add hs3 type veth peer name eth0 netns l2h1
sudo ip -n loxilb link set hs3 mtu 9000 up
sudo ip -n l2h1 link set eth0 mtu 7000 up
sudo ip netns exec l2h1 vconfig add eth0 100
sudo ip netns exec l2h1 ifconfig eth0.100 100.100.100.1/24 up
sudo ip netns exec l2h1 ip route add default via 100.100.100.254

sudo ip -n loxilb link add hs4 type veth peer name eth0 netns l2h2
sudo ip -n loxilb link set hs4 mtu 9000 up
sudo ip -n l2h2 link set eth0 mtu 7000 up
sudo ip netns exec l2h2 vconfig add eth0 100
sudo ip netns exec l2h2 ifconfig eth0.100 100.100.100.2/24 up
sudo ip netns exec l2h2 ip route add default via 100.100.100.254
sudo ip netns exec l2h2 ip addr add 100.100.100.3/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.4/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.5/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.6/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.7/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.8/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.9/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.10/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.11/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.12/24 dev eth0.100
sudo ip netns exec l2h2 ip addr add 100.100.100.13/24 dev eth0.100

sudo ip netns exec loxilb brctl addbr hsvlan100
sudo ip netns exec loxilb vconfig add hs3 100
sudo ip netns exec loxilb vconfig add hs4 100
sudo ip netns exec loxilb brctl addif hsvlan100 hs3.100
sudo ip netns exec loxilb brctl addif hsvlan100 hs4.100
sudo ip netns exec loxilb ifconfig hs3.100 up
sudo ip netns exec loxilb ifconfig hs4.100 up
sudo ip netns exec loxilb ifconfig hsvlan100 100.100.100.254/24 up

sudo ip -n loxilb link add hs5 type veth peer name eth0 netns l2vxh1
sudo ip -n loxilb link set hs5 mtu 9000 up
sudo ip -n l2vxh1 link set eth0 mtu 7000 up
sudo ip netns exec l2vxh1 ifconfig eth0 50.50.50.1/24 up
sudo ip netns exec l2vxh1 ethtool -K eth0 rxvlan off txvlan off

sudo ip -n loxilb link add hs6 type veth peer name eth0 netns l2vxh2
sudo ip -n loxilb link set hs6 mtu 9000 up
sudo ip -n l2vxh2 link set eth0 mtu 7000 up
sudo ip netns exec l2vxh2 ifconfig eth0 2.2.2.2/24 up
sudo ip netns exec l2vxh2 ip link add vxlan50 type vxlan id 50 local 2.2.2.2 dev eth0 dstport 4789
sudo ip netns exec l2vxh2 ifconfig vxlan50 50.50.50.2/24 up
sudo ip netns exec l2vxh2 bridge fdb append 00:00:00:00:00:00 dst 2.2.2.1 dev vxlan50
sudo ip netns exec l2vxh2 ethtool -K eth0 rxvlan off txvlan off

sudo ip netns exec loxilb brctl addbr hsvlan20
sudo ip netns exec loxilb brctl addif hsvlan20 hs6
sudo ip netns exec loxilb ip link set hsvlan20 up
sudo ip netns exec loxilb ip addr add 2.2.2.1/24 dev hsvlan20
sudo ip netns exec loxilb ip link add hsvxlan50 type vxlan id 50 local 2.2.2.1 dev hsvlan20 dstport 4789
sudo ip netns exec loxilb ip link set hsvxlan50 up
sudo ip netns exec loxilb bridge fdb append 00:00:00:00:00:00 dst 2.2.2.2 dev hsvxlan50
sudo ip netns exec loxilb brctl addbr hsvlan50
sudo ip netns exec loxilb brctl addif hsvlan50 hsvxlan50
sudo ip netns exec loxilb brctl addif hsvlan50 hs5
sudo ip netns exec loxilb ip link set hsvlan50 up


## Setup vxlan access port as trunk in l2vxh1 and also corresponding underlay interface of hsvxlan51 as trunk

# Setup l2vxh1
sudo ip netns exec l2vxh1 vconfig add eth0 51
sudo ip netns exec l2vxh1 ifconfig eth0.51 51.51.51.1/24 up

# Setup l2vxh2
sudo ip netns exec l2vxh2 vconfig add eth0 30
sudo ip netns exec l2vxh2 ifconfig eth0.30 3.3.3.2/24 up
sudo ip netns exec l2vxh2 ip link add vxlan51 type vxlan id 51 local 3.3.3.2 dev eth0.30 dstport 4789
sudo ip netns exec l2vxh2 ifconfig vxlan51 51.51.51.2/24 up
sudo ip netns exec l2vxh2  bridge fdb append 00:00:00:00:00:00 dst 3.3.3.1 dev vxlan51

# Setup loxilb hsvxlan51
sudo ip netns exec loxilb brctl addbr hsvlan30
sudo ip netns exec loxilb vconfig add hs6 30
sudo ip netns exec loxilb ip link set hs6.30 up
sudo ip netns exec loxilb brctl addif hsvlan30 hs6.30
sudo ip netns exec loxilb ip link set hsvlan30 up
sudo ip netns exec loxilb ip addr add 3.3.3.1/24 dev hsvlan30
sudo ip netns exec loxilb ip link add hsvxlan51 type vxlan id 51 local 3.3.3.1 dev hsvlan30 dstport 4789
sudo ip netns exec loxilb ip link set hsvxlan51 up
sudo ip netns exec loxilb bridge fdb append 00:00:00:00:00:00 dst 3.3.3.2 dev hsvxlan51
sudo ip netns exec loxilb brctl addbr hsvlan51
sudo ip netns exec loxilb vconfig add hs5 51
sudo ip netns exec loxilb ip link set hs5.51 up
sudo ip netns exec loxilb brctl addif hsvlan51 hsvxlan51
sudo ip netns exec loxilb brctl addif hsvlan51 hs5.51
sudo ip netns exec loxilb ip link set hsvlan51 up
sudo ip netns exec loxilb bridge fdb add to 06:02:02:03:04:06 dst 3.3.3.2 dev hsvxlan51
sudo ip netns exec loxilb bridge fdb add to 06:02:02:03:04:06 dst 3.3.3.2 dev hsvxlan51 master

#Setup l3vxh1
sudo ip -n loxilb link add hs7 type veth peer name eth0 netns l3vxh1
sudo ip -n loxilb link set hs7 mtu 9000 up
sudo ip -n l3vxh1 link set eth0 mtu 7000 up
sudo ip netns exec l3vxh1 ifconfig eth0 17.17.17.1/24 up
sudo ip netns exec l3vxh1 ip route add default via 17.17.17.254

#Set loxilb
sudo ip netns exec loxilb ifconfig hs7 17.17.17.254/24 up
sudo ip -n loxilb link add hs8 type veth peer name eth0 netns l3vxh2
sudo ip -n loxilb link set hs8 mtu 9000 up
sudo ip -n l3vxh2 link set eth0 mtu 7000 up
sudo ip -n loxilb link add hs9 type veth peer name eth1 netns l3vxh2
sudo ip -n loxilb link set hs9 mtu 9000 up
sudo ip -n l3vxh2 link set eth1 mtu 7000 up
sudo ip -n loxilb link add hs10 type veth peer name eth2 netns l3vxh2
sudo ip -n loxilb link set hs10 mtu 9000 up
sudo ip -n l3vxh2 link set eth1 mtu 7000 up

sudo ip netns exec loxilb ip link add hsbond1 type bond
sudo ip netns exec loxilb ip link set hsbond1 type bond mode 802.3ad

sudo ip netns exec loxilb ip link set hs9 down
sudo ip netns exec loxilb ip link set hs10 down
sudo ip netns exec loxilb ip link set hs9 master hsbond1
sudo ip netns exec loxilb ip link set hs10 master hsbond1
sudo ip netns exec loxilb ip link set hs9 mtu 9000 up
sudo ip netns exec loxilb ip link set hs10 mtu 9000 up
sudo ip netns exec loxilb ip link set hsbond1 mtu 9000 up

sudo ip netns exec loxilb brctl addbr hsvlan8
#sudo ip netns exec loxilb brctl addif hsvlan8 hs8
sudo ip netns exec loxilb brctl addif hsvlan8 hsbond1
sudo ip netns exec loxilb ip link set hsvlan8 up
sudo ip netns exec loxilb ip addr add 8.8.8.254/24 dev hsvlan8
sudo ip netns exec loxilb ip link add hsvxlan78 type vxlan id 78 local 8.8.8.254 dev hsvlan8 dstport 4789
sudo ip netns exec loxilb ip link set hsvxlan78 up
sudo ip netns exec loxilb ifconfig hsvxlan78 78.78.78.254/24 up
sudo ip netns exec loxilb bridge fdb append 00:00:00:00:00:00 dst 8.8.8.1 dev hsvxlan78

#Setup l3vxh2
sudo ip netns exec l3vxh2 ip link add bond1 type bond
sudo ip netns exec l3vxh2 ip link set bond1 type bond mode 802.3ad
sudo ip netns exec l3vxh2 ip link set eth1 down
sudo ip netns exec l3vxh2 ip link set eth2 down
sudo ip netns exec l3vxh2 ip link set eth1 master bond1
sudo ip netns exec l3vxh2 ip link set eth2 master bond1
sudo ip netns exec l3vxh2 ip link set eth1 up
sudo ip netns exec l3vxh2 ip link set eth2 up
sudo ip netns exec l3vxh2 ip link set bond1 up
#sudo ip netns exec l3vxh2 ifconfig eth0 8.8.8.1/24 up
sudo ip netns exec l3vxh2 ifconfig bond1 8.8.8.1/24 up
sudo ip netns exec l3vxh2 ip link add vxlan78 type vxlan id 78 local 8.8.8.1 dev bond1 dstport 4789
sudo ip netns exec l3vxh2 ifconfig vxlan78 78.78.78.1/24 up
sudo ip netns exec l3vxh2 ip addr add 18.18.18.1/24 dev vxlan78
sudo ip netns exec l3vxh2  bridge fdb append 00:00:00:00:00:00 dst 8.8.8.254 dev vxlan78
sudo ip netns exec l3vxh2 ip route add default via 78.78.78.254
sudo ip netns exec loxilb ip route add 18.18.18.0/24 via 78.78.78.1

sudo mkdir -p /opt/netlox/loxilb/
sudo mount -t bpf bpf /opt/netlox/loxilb/
