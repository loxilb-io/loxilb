#!/bin/bash
vagrant global-status  | grep -i virtualbox | cut -f 1 -d ' ' | xargs -L 1 vagrant destroy -f
vagrant up
vagrant ssh host -c 'sudo ip route add 123.123.123.0/24 via 192.168.90.9'

