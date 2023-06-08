#!/bin/bash
sudo ip route del 123.123.123.1 via 192.168.90.9
vagrant destroy worker1
vagrant destroy master
vagrant destroy loxilb
