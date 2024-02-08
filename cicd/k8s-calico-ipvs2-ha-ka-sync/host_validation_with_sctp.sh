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
stdbuf -oL nohup iperf -c  192.168.80.5 -p 56002 -t 100 -i 1 -b 100M &> iperff.out &

echo "iperf client started"

sleep 1

mkfifo sd1.pipe

sleep infinity > sd1.pipe &

stdbuf -oL nohup sctp_darn -H 192.168.80.9 -h 192.168.80.5 -p 56004 -s -I < sd1.pipe &> sdf.out &

echo "sctp_test client started"

sleep 2
for((i=0;i<30;i++))
do
echo "snd=100" 1> sd1.pipe
sleep 1
done
echo "phase-1 done"
exit 0
