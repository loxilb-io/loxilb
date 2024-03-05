#!/bin/bash
extIP=$(cat /vagrant/extIP)

code=0

echo Service IP: $extIP

neigh=$( ip neigh | grep $extIP )

if [[ ! -z $neigh && $neigh != *"FAILED"* ]]; then
    echo "Host route [OK]"
else
    echo "Host route [NOK]"
fi
echo -e "\n*********************************************"
echo "Testing Service"
echo "*********************************************"

# iperf client accessing fullnat service
stdbuf -oL nohup iperf -c 192.168.80.5 -p 56002 -t 100 -i 1 -b 100M &> iperff.out &
echo "iperf client started"
echo "phase-1 done"
