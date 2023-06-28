#!/bin/bash
for iface in $(ifconfig | cut -d ' ' -f1| tr ':' '\n' | awk NF); do printf "$iface%s\n"; sudo ntc filter del dev $iface ingress; sudo ip link set dev $iface xdp off;sudo ip link set dev $iface xdpgeneric off; done
