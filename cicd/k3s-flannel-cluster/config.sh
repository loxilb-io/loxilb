#!/bin/bash
vagrant global-status --prune
vagrant up
sudo ip route add 123.123.123.1 via 192.168.90.9
