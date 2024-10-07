#!/bin/bash
source ../common.sh
source check_ha.sh
echo -e "sctpmh: SCTP Multihoming - E2E Multipath Failover Test. Client, LB and EP all Multihomed\n"
extIP="133.133.133.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

echo -e "\nHA state Master:$master BACKUP-$backup\n"
echo -e "\nTraffic Flow: EP ---> LB ---> User"

$hexec user sctp_test -H 0.0.0.0  -P 9999 -l > user.out &
sleep 2

$hexec ep1 stdbuf -oL sctp_test -H 31.31.31.1 -B 32.32.32.1 -P 20000 -h $extIP -p $port -s -c 6 -x 1000 > ep1.out &

#Path counters
p1c_old=0
p1c_new=0
p2c_old=0
p2c_new=0
p3c_old=0
p3c_new=0
down=0
code=0
sleep 2

for((i=0;i<200;i++)) do
    fin=`tail -n 100 ep1.out | grep "Client: Sending packets.(1000/1000)"`
    if [[ ! -z $fin ]]; then
        fin=1
        echo "sctp_test done."
        break;
    fi
    $dexec $master loxicmd get ct --servName=sctpmh2
    echo -e "\n"
    p1c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "133.133.133.1 | 31.31.31.1" | xargs | cut -d '|' -f 10)
    p2c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "134.134.134.1 | 32.32.32.1" | xargs | cut -d '|' -f 10)
    p3c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "135.135.135.1 | 31.31.31.1" | xargs | cut -d '|' -f 10)
    
    echo "Counters: $p1c_new $p2c_new $p3c_new"

    if [[ $p1c_new -gt $p1c_old ]]; then
        echo "Path 1: 31.31.31.1 -> 133.133.133.1 -> 1.1.1.1 [ACTIVE]"
        p1=1
        #if [[ $down == 1 ]]; then
        #    echo "This path shouldn't be ACTIVE"
        #    code=1
        #fi
        echo -e "Turning off this path at User.\nEP----->LB--x-->User"
        $hexec user ip link set euserr1 down;
        down=1
        p1c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh2 | grep "133.133.133.1 | 31.31.31.1" | xargs | cut -d '|' -f 10)
    else
        if [[ $down == 1 ]]; then
            p1dok=1
            echo "Path 1: 31.31.31.1 -> 133.133.133.1 -> 1.1.1.1 NOT ACTIVE - [OK]"
        else  
            echo "Path 1: 31.31.31.1 -> 133.133.133.1 -> 1.1.1.1 [NOT ACTIVE]"
        fi
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
    p1c_old=$p1c_new
    p2c_old=$p1c_new
    p2c_old=$p1c_new
    echo -e "\n"
    sleep 5
done

sudo rm -rf *.out
sudo pkill sctp_test

#Restore
$hexec user ip link set euserr1 up
$hexec user ip route add default via 1.1.1.254

if [[ $fin == 1 && $p1 == 1 && $p2 == 1 && $p3 == 1 && $code == 0 ]]; then
    echo "sctpmh SCTP Multihoming E2E Multipath Failover [OK]"
    echo "OK" > status4.txt
    restart_loxilbs
else
    echo "NOK" > status4.txt
    echo "sctpmh SCTP Multihoming E2E Multipath Failover [NOK]"
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
