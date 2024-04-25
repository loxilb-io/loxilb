#!/bin/bash
source ../common.sh
source check_ha.sh

echo -e "sctpmh: SCTP Multihoming - HA Failover Test. Client and LB Multihomed, EP is uni-homed\n"
extIP="123.123.123.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

echo -e "\nHA state Master:$master BACKUP-$backup\n"

echo -e "\nTraffic Flow: User -> LB -> EP "

$hexec ep1 sctp_test -H 0.0.0.0  -P 9999 -l > ep1.out &
sleep 2

$hexec user stdbuf -oL sctp_test -H 1.1.1.1 -B 2.2.2.1 -P 20000 -h $extIP -p $port -s -m 100 -x 200000 > user.out &

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

function restart_mloxilb() {
    if [[ $master == "llb1" ]]; then
        pat="cluster=172.17.0.3"
        copts=" --cluster=172.17.0.3"
        self=" --self=0"
        ka=" --ka=172.17.0.3:172.17.0.2"
    else
        pat="cluster=172.17.0.2"
        copts=" --cluster=172.17.0.2"
        self=" --self=1"
        ka=" --ka=172.17.0.2:172.17.0.3"
    fi
    pid=$(docker exec -i $master ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    echo Killing $pid >&2
    docker exec -dt $master kill -9 $pid
    docker exec -dt $master ip link del llb0
    docker exec -dt $master nohup /root/loxilb-io/loxilb/loxilb $copts $self $ka > /dev/null &
    pid=$(docker exec -i $master ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    echo "New loxilb pid: $pid" >&2
}

for((i=0;i<200;i++)) do
    fin=`tail -n 100 user.out | grep "Client: Sending packets.(200000/200000)"`
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
        if [[ $nsyncOk == 2 ]]; then
            break;
        fi
    fi
    $dexec $master loxicmd get ct --servName=sctpmh1 
    echo -e "\n"
    p1c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 | grep "123.123.123.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    p2c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 | grep "124.124.124.1 | 2.2.2.1" | xargs | cut -d '|' -f 10)
    p3c_new=$(sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 | grep "125.125.125.1 | 1.1.1.1" | xargs | cut -d '|' -f 10)
    
    echo "Counters: $p1c_new $p2c_new $p3c_new"
    if [[ $p1c_new -gt $p1c_old ]]; then
        echo "Path 1: 1.1.1.1 -> 123.123.123.1 [ACTIVE]"
        p1=1
    else
        echo "Path 1: 1.1.1.1 -> 123.123.123.1 [NOT ACTIVE]"
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

if [[ $fin == 1 && $p1 == 1 && $p2 == 1 && $p3 == 1 && $code == 0 && $syncOk == 1 ]]; then
    echo "sctpmh SCTP Multihoming HA Failover [OK]"
else
    echo "sctpmh SCTP Multihoming HA Failover [NOK]"
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
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
