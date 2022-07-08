#!/bin/bash

LBNSLNCMD="sudo ip -n loxilb link "
IPNSEXE="sudo ip netns exec "
IPNSDEL="sudo ip netns del "
IPNSCMD="sudo ip -n "

$LBNSLNCMD set enp1 down
$IPNSCMD l3h1 link set eth0 down

$LBNSLNCMD set enp2 down
$IPNSCMD l3h2 link set eth0 down

$LBNSLNCMD set enp3 down
$IPNSCMD l2h1 link set eth0 down

$LBNSLNCMD set enp4 down
$IPNSCMD l2h2 link set eth0 down

$LBNSLNCMD set enp5 down
$IPNSCMD l2vxh1 link set eth0 down

$LBNSLNCMD set enp6 down
$IPNSCMD l2vxh2 link set eth0 down

$IPNSEXE l2vxh1 vconfig rem eth0.51
$IPNSEXE l2vxh2 vconfig rem eth0.30

$LBNSLNCMD del vxlan50
$LBNSLNCMD del vxlan51
$LBNSLNCMD del enp1 
$LBNSLNCMD del enp2 
$LBNSLNCMD del enp3 
$LBNSLNCMD del enp4 
$LBNSLNCMD del enp5 
$LBNSLNCMD del enp6 
$LBNSLNCMD del enp7 
$LBNSLNCMD del enp8 
$LBNSLNCMD del enp9 
$LBNSLNCMD del enp10 
$LBNSLNCMD del enp2v15
$LBNSLNCMD del bond1
$LBNSLNCMD del vlan100
$LBNSLNCMD del vlan20
$LBNSLNCMD del vlan30
$LBNSLNCMD del vlan50
$LBNSLNCMD del vlan51
$LBNSLNCMD del vlan8

$IPNSCMD l3h1 link del eth0
$IPNSCMD l3h2 link del eth0
$IPNSCMD l3h3 link del eth0
$IPNSCMD l2h1 link del eth0
$IPNSCMD l2h2 link del eth0
$IPNSCMD l2vxh1 link del eth0
$IPNSCMD l2vxh2 link del eth0
$IPNSCMD l3vxh1 link del eth0
$IPNSCMD l3vxh2 link del eth1
$IPNSCMD l3vxh2 link del eth2

#$IPNSDEL loxilb
$IPNSDEL l3vxh1
$IPNSDEL l3vxh2
$IPNSDEL l2vxh1
$IPNSDEL l2vxh2
$IPNSDEL l2h1
$IPNSDEL l2h2
$IPNSDEL l3h1
$IPNSDEL l3h2
$IPNSDEL l3h3
