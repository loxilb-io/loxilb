#!/bin/bash

sudo ip -n loxilb link set enp1 down
sudo ip -n l3h1 link set eth0 down

sudo ip -n loxilb link set enp2 down
sudo ip -n l3h2 link set eth0 down

sudo ip -n loxilb link set enp3 down
sudo ip -n l2h1 link set eth0 down

sudo ip -n loxilb link set enp4 down
sudo ip -n l2h2 link set eth0 down

sudo ip -n loxilb link set enp5 down
sudo ip -n l2vxh1 link set eth0 down

sudo ip -n loxilb link set enp6 down
sudo ip -n l2vxh2 link set eth0 down

sudo ip netns exec l2vxh1 vconfig rem eth0.51
sudo ip netns exec l2vxh2 vconfig rem eth0.30

sudo ip -n loxilb link del enp1 
sudo ip -n loxilb link del enp2 
sudo ip -n loxilb link del enp3 
sudo ip -n loxilb link del enp4 
sudo ip -n loxilb link del enp5 
sudo ip -n loxilb link del enp6 
sudo ip -n loxilb link del enp7 
sudo ip -n loxilb link del enp8 
sudo ip -n loxilb link del enp9 
sudo ip -n loxilb link del enp10 
sudo ip -n loxilb link del enp2v15

sudo ip -n l3h1 link del eth0
sudo ip -n l3h2 link del eth0
sudo ip -n l3h3 link del eth0
sudo ip -n l2h1 link del eth0
sudo ip -n l2h2 link del eth0
sudo ip -n l2vxh1 link del eth0
sudo ip -n l2vxh2 link del eth0
sudo ip -n l3vxh1 link del eth0
sudo ip -n l3vxh2 link del eth1
sudo ip -n l3vxh2 link del eth2
sudo ip -n l4vxh2 link del eth0

sudo ip netns del loxilb
sudo ip netns del l3vxh1
sudo ip netns del l3vxh2
sudo ip netns del l2vxh1
sudo ip netns del l2vxh2
sudo ip netns del l2h1
sudo ip netns del l2h2
sudo ip netns del l3h1
sudo ip netns del l3h2
sudo ip netns del l3h3
