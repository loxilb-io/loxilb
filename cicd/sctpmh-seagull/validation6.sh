#!/bin/bash
source /vagrant/common.sh
source /vagrant/check_ha.sh

echo -e "sctpmh: SCTP Multihoming - E2E HA Failover Test. Client, LB Multihomed and EP all multi-homed\n"
extIP="133.133.133.1"
port=2020

check_ha

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"

echo -e "\nHA state Master:$master BACKUP-$backup\n"

echo -e "\nTraffic Flow: EP ---> LB ---> User"

sudo docker exec -dt user ksh -c 'sed -i 's/source=31.31.31.1/source=0.0.0.0/g' /opt/seagull/diameter-env/config/conf.server.xml'

sudo docker exec -dt user ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 220 stdbuf -oL seagull -conf ../config/conf.server.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.server.xml > user.out' 2>&1 > /dev/null &
sleep 2

sudo docker exec -dt ep1 ksh -c 'sed -i 's/dest=20.20.20.1/dest=133.133.133.1/g' /opt/seagull/diameter-env/config/conf.client.xml'
sudo docker exec -dt ep1 ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 200 stdbuf -oL seagull -conf ../config/conf.client.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.client.xml > ep1.out' 2>&1 > /dev/null &


sleep 20

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
frecover=1

for((i=0;i<15;i++)) do
    syncOk=$nsyncOk
    if [[ $checkha == 1 ]]; then
        check_ha
        echo -e "\nHA state Master:$master BACKUP-$backup\n"
        nsyncOk=$(checkSync)
        if [[ $nsyncOk == 2 ]]; then #No active connections in Master, no need to continue.
            break;
        fi
    fi
    $dexec ep1 bash -c 'tail -n 25 /opt/seagull/diameter-env/run/ep1.out'
    call_new=$(sudo docker exec -t ep1 bash -c 'tail -n 10 /opt/seagull/diameter-env/run/ep1.out | grep "Successful calls"'| xargs | cut -d '|' -f 4)
    fail_new=$(sudo docker exec -t ep1 bash -c 'tail -n 10 /opt/seagull/diameter-env/run/ep1.out | grep "Failed calls"'| xargs | cut -d '|' -f 4)
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
	if [[ $recover == 1 ]]; then
        printf "\t***Setup Recovered***"
    fi
    echo -e "\n\n"

    
    if [[ $fail_new -gt $fail_old && $hadone == 1 && $recover == 0 ]]; then
	    echo -e "Failed Calls:   \t\t$fail_new \t\t[INCREASING]"
	    fstart=1
        frecover=0
        calls=0
        code=1
    else 
        if [[ $fail_new -eq $fail_old ]]; then
            if [[ $hadone == 1 ]]; then
	            printf "Failed Calls:   \t%10s \t[STABLE]\n" $fail_new
	            frecover=1
                code=0
            else
	            printf "Failed Calls:   \t%10s\n" $fail_new
	        fi
        fi
    fi

    if [[ $call_new -gt $call_old ]]; then
	    printf "Successful Calls: \t%10s \t[ACTIVE]\n" $call_new
        calls=1
	    if [[ $hadone == 1 && $frecover == 1 ]]; then
            recover=1
	    fi
    else 
	    printf "Successful Calls: \t%10s \t[NOT ACTIVE]\n" $call_new
    fi

    p1c_old=$p1c_new
    p2c_old=$p1c_new
    p2c_old=$p1c_new
    call_old=$call_new
    fail_old=$fail_new

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

#Revert
sudo docker exec -dt user ksh -c 'sed -i 's/source=0.0.0.0/source=31.31.31.1/g' /opt/seagull/diameter-env/config/conf.server.xml'
sudo docker exec -dt ep1 ksh -c 'sed -i 's/dest=133.133.133.1/dest=20.20.20.1/g' /opt/seagull/diameter-env/config/conf.client.xml'

if [[ $p1 == 1 && $p2 == 1 && $p3 == 1 && $code == 0 && $nsyncOk == 1 && $recover == 1 ]]; then
    echo "sctpmh SCTP Multihoming E2E HA Failover [OK]"
    echo "OK" > /vagrant/status6.txt
    restart_loxilbs
else
    echo "NOK" > /vagrant/status6.txt
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
