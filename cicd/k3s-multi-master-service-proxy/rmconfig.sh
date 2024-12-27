#!/bin/bash
sudo ip route del 123.123.123.1 via 192.168.90.10 || true
vagrant destroy -f worker1
vagrant destroy -f worker2
vagrant destroy -f master1
vagrant destroy -f master2
vagrant destroy -f master3
vagrant destroy -f host
