#!/bin/bash
source /vagrant/common.sh
source /vagrant/check_ha.sh

echo -e "sctpmh: SCTP Multihoming Basic Test - Client & EP Uni-homed and LB is Multi-homed\n"
extIP="20.20.20.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

sudo docker exec -dt ep1 ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 40 stdbuf -oL seagull -conf ../config/conf.server.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.server.xml > ep1.out' 2>&1 > /dev/null &
sleep 2

sudo docker exec -dt user ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 25 stdbuf -oL seagull -conf ../config/conf.client.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.client.xml > user.out' 2>&1 > /dev/null &

sleep 2

for((i=0;i<5;i++)) do
    $dexec user bash -c 'tail -n 25 /opt/seagull/diameter-env/run/user.out'
    res=$(sudo docker exec -t user bash -c 'tail -n 10 /opt/seagull/diameter-env/run/user.out | grep "Successful calls"'| xargs | cut -d '|' -f 4)
    $dexec $master loxicmd get ct --servName=sctpmh1
    echo -e "\n"
    sleep 5
done

if [ "$res" -gt "0" ]; then
    #echo -e $res
    echo -e "\nsctpmh SCTP Multihoming service Basic Test [OK]\n"
    echo "OK" > /vagrant/status1.txt
    restart_loxilbs
else
    echo "NOK" > /vagrant/status1.txt
    echo "sctpmh SCTP Multihoming service Basic Test [NOK]"
    echo "Calls : $res"
    ## Dump some debug info
    echo "system route-info"
    echo -e "\nuser"
    sudo ip netns exec user ip route
    echo -e "\nr1"
    sudo ip netns exec r1 ip route
    echo -e "\nr2"
    sudo ip netns exec r2 ip route
    echo -e "\nllb1"
    sudo ip netns exec llb1 ip route
    echo -e "\nr3"
    sudo ip netns exec r3 ip route
    echo -e "\nr4"
    sudo ip netns exec r4 ip route
    echo "-----------------------------"

    echo -e "\nllb1 lb-info"
    $dexec llb1 loxicmd get lb
    echo "llb1 ep-info"
    $dexec llb1 loxicmd get ep
    echo "llb1 bpf-info"
    $dexec llb1 tc filter show dev eth0 ingress
    echo "BFP trace -- "
    sudo timeout 5 cat  /sys/kernel/debug/tracing/trace_pipe
    sudo killall -9 cat
    echo "BFP trace -- "
    restart_loxilbs
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
