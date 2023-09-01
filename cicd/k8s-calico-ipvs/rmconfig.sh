#!/bin/bash
sudo ip route del 123.123.123.1 via 192.168.90.9
sudo ip route del 124.124.124.1 via 192.168.90.9
sudo ip route del 125.125.125.1 via 192.168.90.9
vagrant destroy -f worker2
vagrant destroy -f worker1
vagrant destroy -f master
vagrant destroy -f loxilb
sudo rm loxilb-ip
