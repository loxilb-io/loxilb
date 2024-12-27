#!/bin/bash
sudo ip route del 123.123.123.1 via 192.168.90.9 || true
vagrant destroy -f worker1
vagrant destroy -f master
vagrant destroy -f loxilb
