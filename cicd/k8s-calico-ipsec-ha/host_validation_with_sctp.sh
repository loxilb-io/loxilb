#!/bin/bash
extIP=$(cat /vagrant/extIP)

code=0

echo Service IP: $extIP

numECMP=$(birdc show route | grep $extIP -A 3 | grep via | wc -l)

birdc show route | grep $extIP -A 3

if [ $numECMP == "2" ]; then
    echo "Host route [OK]"
else
    echo "Host route [NOK]"
fi
echo -e "\n*********************************************"
echo "Testing Service"
echo "*********************************************"

# iperf client accessing fullnat service
stdbuf -oL nohup iperf -c 20.20.20.1 -p 56002 -t 100 -i 1 -b 100M -B 30.30.30.1 &> iperff.out &

# iperf client accessing default service
stdbuf -oL nohup iperf -c 20.20.20.1 -p 56003 -t 100 -i 1 -b 100M -B 30.30.30.1 &> iperfd.out &

echo "iperf client started"

sleep 1

stdbuf -oL nohup sctp_test -H 30.30.30.1 -h 20.20.20.1 -p 56004 -s -m 1400 -x 70000 &> sdf.out &
stdbuf -oL nohup sctp_test -H 30.30.30.1 -h 20.20.20.1 -p 56005 -s -m 1400 -x 70000 &> sdd.out &

echo "sctp_test client started"

echo "phase-1 done"
exit 0
