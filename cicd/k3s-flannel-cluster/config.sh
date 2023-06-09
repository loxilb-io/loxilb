#!/bin/bash
vagrant global-status  | grep -i virtualbox | cut -f 1 -d ' ' | xargs -L 1 vagrant destroy -f
vagrant up
sudo ip route add 123.123.123.1 via 192.168.90.9 || true
