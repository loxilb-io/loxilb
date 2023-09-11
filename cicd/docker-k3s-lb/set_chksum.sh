#!/bin/bash
dname=$1
mode=$2
if [ "x"$dname == "x" ]; then
  echo "No argument specified.Please provide docker name as arg1"
  exit
fi
if [ "x"$mode == "x" ]; then
  echo "No mode argument specified.Please provide mode: on or off as arg2"
  exit
fi
if [ $mode != "off" -a $mode != "on" ]; then
  echo "Please provide mode: on or off"
  exit
fi
id=`sudo docker ps -f name=$1| grep -w $1 | cut  -d " "  -f 1 | grep -iv  "CONTAINER"`
pid=`sudo docker inspect -f '{{.State.Pid}}' $id`
echo $pid
if [ ! -f "/var/run/netns/$dname" -a "$pid" != "" ]; then
    sudo mkdir -p /var/run/netns
    sudo touch /var/run/netns/$dname
    sudo mount -o bind /proc/$pid/ns/net /var/run/netns/$dname
fi
sudo ip netns exec $dname ethtool -K eth0 tx $mode
