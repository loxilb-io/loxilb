#!/bin/bash
source /vagrant/common.sh
source /vagrant/check_ha.sh
echo -e "sctpmh: SCTP Multihoming - Multipath Test, Client and LB Multihomed, EP is uni-homed\n"
extIP="20.20.20.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

echo -e "\nHA state Master:$master BACKUP-$backup\n"

sudo docker exec -dt ep1 ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 40 stdbuf -oL seagull -conf ../config/conf.server.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.server.xml > ep1.out' 2>&1 > /dev/null &
sleep 2

sudo docker exec -dt user ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 25 stdbuf -oL seagull -conf ../config/conf.client.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.client.xml > user.out' 2>&1 > /dev/null &

sleep 2
#Path counters
p1c_old=0
p1c_new=0
p2c_old=0
p2c_new=0
p3c_old=0
p3c_new=0
call_old=0
call_new=0

for((i=0;i<5;i++)) do

    $dexec user bash -c 'tail -n 25 /opt/seagull/diameter-env/run/user.out'
    call_new=$(sudo docker exec -t user bash -c 'tail -n 10 /opt/seagull/diameter-env/run/user.out | grep "Successful calls"'| xargs | cut -d '|' -f 4)
    echo -e "\n\n"
    $dexec $master loxicmd get ct --servName=sctpmh1
    echo -e "\n"
    p1c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 | grep "20.20.20.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    p2c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 | grep "21.21.21.1 | 2.2.2.1" | xargs | cut -d '|' -f 10)
    p3c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 | grep "22.22.22.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    
    echo "Counters: $p1c_new $p2c_new $p3c_new"

    if [[ $p1c_new -gt $p1c_old ]]; then
        echo "Path 1: 1.1.1.1 -> 20.20.20.1 [ACTIVE]"
        p1=1
    else
        echo "Path 1: 1.1.1.1 -> 20.20.20.1 [NOT ACTIVE]"
    fi

    if [[ $p2c_new -gt $p2c_old ]]; then
        echo "Path 2: 2.2.2.1 -> 21.21.21.1 [ACTIVE]"
        p2=1
    else
        echo "Path 2: 2.2.2.1 -> 21.21.21.1 [NOT ACTIVE]"
    fi

    if [[ $p3c_new -gt $p3c_old ]]; then
        echo "Path 3: 1.1.1.1 -> 22.22.22.1 [ACTIVE]"
        p3=1
    else
        echo "Path 3: 1.1.1.1 -> 22.22.22.1 [NOT ACTIVE]"
    fi
    if [[ $call_new -gt $call_old ]]; then
	    echo "\nSuccessful Calls: \t$call_new [ACTIVE]"
        calls=1
    else
        echo "\nSuccessful Calls: \t$call_new [NOT ACTIVE]"
    fi

    p1c_old=$p1c_new
    p2c_old=$p1c_new
    p2c_old=$p1c_new
    call_old=$call_new
    echo -e "\n"
    sleep 5
done

if [[ $p1 == 1 && $p2 == 1 && $p3 == 1 && $calls == 1 ]]; then
    echo "sctpmh SCTP Multihoming Multipath [OK]"
    echo "OK" > /vagrant/status2.txt
    restart_loxilbs
else
    echo "NOK" > /vagrant/status2.txt
    echo "sctpmh SCTP Multihoming Multipath [NOK]"
    echo -e "\nuser"
    sudo ip netns exec user ip route
    echo -e "\nr1"
    sudo ip netns exec r1 ip route
    echo -e "\nr2"
    sudo ip netns exec r2 ip route
    echo -e "\nllb1"
    sudo ip netns exec llb1 ip route
    echo -e "\nllb2"
    sudo ip netns exec llb2 ip route
    echo -e "\nr3"
    sudo ip netns exec r3 ip route
    echo -e "\nr4"
    sudo ip netns exec r4 ip route
    echo "-----------------------------"

    echo -e "\nllb1 lb-info"
    $dexec llb1 loxicmd get lb
    echo "llb1 ep-info"
    $dexec llb1 loxicmd get ep
    echo "-----------------------------"
    echo -e "\nllb2 lb-info"
    $dexec llb2 loxicmd get lb
    echo "llb2 ep-info"
    $dexec llb2 loxicmd get ep
    restart_loxilbs
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
