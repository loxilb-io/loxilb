#!/bin/bash
source ../common.sh

echo -e "sctpmh: SCTP Multihoming - Multipath Failover Test. Client and LB Multihomed, EP is uni-homed\n"
extIP="123.123.123.1"
port=2020

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

echo -e "\nTraffic Flow: User -> LB -> EP "

$hexec ep1 sctp_test -H 0.0.0.0  -P 9999 -l > ep1.out &
sleep 2

$hexec user stdbuf -oL sctp_test -H 1.1.1.1 -B 2.2.2.1 -P 20000 -h $extIP -p $port -s -m 100 -x 100000 > user.out &
sleep 5
#Path counters
p1c_old=0
p1c_new=0
p2c_old=0
p2c_new=0
p3c_old=0
p3c_new=0
down=0
code=0
for((i=0;i<200;i++)) do
    fin=`tail -n 100 user.out | grep "Client: Sending packets.(100000/100000)"`
    if [[ ! -z $fin ]]; then
        fin=1
        echo "sctp_test done."
        break;
    fi
    $dexec llb1 loxicmd get ct
    echo -e "\n"
    p1c_new=$(sudo docker exec -i llb1 loxicmd get ct | grep "123.123.123.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    p2c_new=$(sudo docker exec -i llb1 loxicmd get ct | grep "124.124.124.1 | 2.2.2.1" | xargs | cut -d '|' -f 10)
    p3c_new=$(sudo docker exec -i llb1 loxicmd get ct | grep "125.125.125.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    
    echo "Counters: $p1c_new $p2c_new $p3c_new"

    if [[ $p1c_new -gt $p1c_old ]]; then
        echo "Path 1: 1.1.1.1 -> 123.123.123.1 [ACTIVE]"
        p1=1
        if [[ $down == 1 ]]; then
            echo "This path shouldn't be ACTIVE"
            code=1
        fi
        echo "Turning off this path from User->LB"
        $hexec user ip link set euserr1 down;
        down=1
        p1c_new=$(sudo docker exec -i llb1 loxicmd get ct | grep "123.123.123.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    else
        if [[ $down == 1 ]]; then
            p1dok=1
            echo "Path 1: 1.1.1.1 -> 123.123.123.1 NOT ACTIVE - [OK]"
        else  
            echo "Path 1: 1.1.1.1 -> 123.123.123.1 [NOT ACTIVE]"
        fi
    fi

    if [[ $p2c_new -gt $p2c_old ]]; then
        echo "Path 2: 2.2.2.1 -> 124.124.124.1 [ACTIVE]"
        p2=1
    else
        echo "Path 2: 2.2.2.1 -> 124.124.124.1 [NOT ACTIVE]"
    fi

    if [[ $p3c_new -gt $p3c_old ]]; then
        echo "Path 3: 1.1.1.1 -> 125.125.125.1 [ACTIVE]"
        p3=1
    else
        echo "Path 3: 1.1.1.1 -> 125.125.125.1 [NOT ACTIVE]"
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

if [[ $fin == 1 && $p1 == 1 && $p2 == 1 && $p3 == 1 && $p1dok == 1 && $code == 0 ]]; then
    echo "sctpmh SCTP Multihoming Multipath Failover [OK]"
else
    echo "sctpmh SCTP Multihoming Multipath Failover [NOK]"
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
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
