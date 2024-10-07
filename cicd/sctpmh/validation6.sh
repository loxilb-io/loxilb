#!/bin/bash
source ../common.sh
source check_ha.sh

echo -e "sctpmh: SCTP Multihoming - E2E HA Failover Test. Client, LB and EP all Multi-homed\n"
extIP="133.133.133.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

echo -e "\nHA state Master:$master BACKUP-$backup\n"

echo -e "\nTraffic Flow: EP ---> LB ---> User"

sudo pkill sctp_test

#$hexec user sctp_test -H 1.1.1.1 -B 2.2.2.1  -P 9999 -l > user.out &
$hexec user sctp_test -H 0.0.0.0  -P 9999 -l > user.out &
sleep 2

$hexec ep1 stdbuf -oL sctp_test -H 31.31.31.1 -B 32.32.32.1 -P 20000 -h $extIP -p $port -s -c 6 -x 10000 > ep1.out &

#Path counters
p1c_old=0
p1c_new=0
p2c_old=0
p2c_new=0
p3c_old=0
p3c_new=0
checkha=0
hadone=0
code=0
nsyncOk=0

for((i=0;i<500;i++)) do
    fin=`tail -n 100 ep1.out | grep "Client: Sending packets.(10000/10000)"`
    if [[ ! -z $fin ]]; then
        fin=1
        echo "sctp_test done."
        break;
    fi

    syncOk=$nsyncOk
    if [[ $checkha == 1 ]]; then
        check_ha
        echo -e "\nHA state Master:$master BACKUP-$backup\n"
        nsyncOk=$(checkSync)
        if [[ $nsyncOk == 2 ]]; then #No active connections in Master, no need to continue.
            break;
        fi
    fi
    echo -e "\n"
    $dexec $master loxicmd get ct --servName=sctpmh2
    echo -e "\n"
    p1c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "133.133.133.1 | 31.31.31.1" | xargs | cut -d '|' -f 10)
    p2c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "134.134.134.1 | 32.32.32.1" | xargs | cut -d '|' -f 10)
    p3c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "135.135.135.1 | 31.31.31.1" | xargs | cut -d '|' -f 10)
    
 
    echo "Counters: $p1c_new $p2c_new $p3c_new"
    if [[ $p1c_new -gt $p1c_old ]]; then
        echo "Path 1: 31.31.31.1 -> 133.133.133.1 -> 1.1.1.1 [ACTIVE]"
        p1=1
    else
        echo "Path 1: 31.31.31.1 -> 133.133.133.1 -> 1.1.1.1 [NOT ACTIVE]"
    fi

    if [[ $p2c_new -gt $p2c_old ]]; then
        echo "Path 2: 32.32.32.1 -> 134.134.134.1 -> 2.2.2.1 [ACTIVE]"
        p2=1
    else
        echo "Path 2: 32.32.32.1 -> 134.134.134.1 -> 2.2.2.1 [NOT ACTIVE]"
    fi

    if [[ $p3c_new -gt $p3c_old ]]; then
        echo "Path 3: 31.31.31.1 -> 135.135.135.1 -> 1.1.1.1 [ACTIVE]"
        p3=1
    else
        echo "Path 3: 31.31.31.1 -> 135.135.135.1 -> 1.1.1.1 [NOT ACTIVE]"
    fi
    echo -e "\n"
    
    p1c_old=$p1c_new
    p2c_old=$p1c_new
    p2c_old=$p1c_new

    if [[ $hadone == 0 ]]; then
        nsyncOk=$(checkSync)
        if [[ $nsyncOk == 1 ]]; then
            restart_mloxilb
            checkha=1
            hadone=1
        fi
    fi
    sleep 5
done


sudo rm -rf *.out
sudo pkill sctp_test

if [[ $fin == 1 && $p1 == 1 && $p2 == 1 && $p3 == 1 && $code == 0 && $nsyncOk == 1 ]]; then
    echo "sctpmh SCTP Multihoming E2E HA Failover [OK]"
    echo "OK" > status6.txt
    restart_loxilbs
else
    echo "NOK" > status6.txt
    echo "sctpmh SCTP Multihoming E2E HA Failover [NOK]"
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
    echo -e "\nllb2 lb-info"
    $dexec llb2 loxicmd get lb
    echo "llb2 ep-info"
    $dexec llb2 loxicmd get ep
    echo "-----------------------------"
    restart_loxilbs
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
