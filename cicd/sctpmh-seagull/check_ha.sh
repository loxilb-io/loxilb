#!/bin/bash

master="llb1"
backup="llb2"

function check_ha() {
    while : ; do
        status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
        status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
        count=0
        if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
        then
            master="llb1"
            backup="llb2"
            break
        elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; 
        then
            master="llb2"
            backup="llb1"
            break
        else
            count=$(( $count + 1 ))
            if [[ $count -ge 20 ]]; then
                echo "KA llb1-$status1, llb2-$status2 [NOK] - Exiting" >&2
                exit 1;
            fi
            echo "KA llb1-$status1, llb2-$status2 [NOK]" >&2
            sleep 5
        fi
    done
}

function checkSync() {
    count=1
    sync=0
    while [[ $count -le 5 ]] ; do
        echo -e "\nStatus at MASTER:$master\n" >&2
        ct=`$dexec $master loxicmd get ct | grep est`
        echo "${ct//'\n'/$'\n'}" >&2

        echo -e "\nStatus at BACKUP:$backup\n" >&2
        ct=`$dexec $backup loxicmd get ct | grep est`
        echo "${ct//'\n'/$'\n'}" >&2

        nres1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)
        nres2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)

        if [[ $nres1 == 0 ]]; then
            echo -e "No active connections in Master:$master. Exiting!" >&2
            return 2
        fi

        if [[ $nres1 == $nres2 && $nres1 != 0 ]]; then
            echo -e "\nConnections sync successful!!!\n" >&2
            sync=1
            break;
        fi
        echo -e "\nConnections sync pending.. Let's wait a little more..\n" >&2
        count=$(( $count + 1 ))
        sleep 2
    done

    if [[ $sync == 0 ]]; then
        echo -e "\nConnection Sync failed\n" >&2
        return 0
    fi
    echo "$sync"
}

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
    echo "Restarting MASTER: $master"
    pid=$(docker exec -i $master ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    echo "Killing $pid" >&2
    docker exec -dt $master kill -9 $pid
    docker exec -dt $master ip link del llb0
    docker exec -dt $master nohup /root/loxilb-io/loxilb/loxilb $copts $self $ka > /dev/null &
    pid=$(docker exec -i $master ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    echo "New loxilb pid: $pid" >&2
}

function restart_loxilbs() {
    if [[ $master == "llb1" ]]; then
        mpat="cluster=172.17.0.3"
        mcopts=" --cluster=172.17.0.3"
        mself=" --self=0"
        mka=" --ka=172.17.0.3:172.17.0.2"
        
        bpat="cluster=172.17.0.2"
        bcopts=" --cluster=172.17.0.2"
        bself=" --self=1"
        bka=" --ka=172.17.0.2:172.17.0.3"
    else
        mpat="cluster=172.17.0.2"
        mcopts=" --cluster=172.17.0.2"
        mself=" --self=1"
        mka=" --ka=172.17.0.2:172.17.0.3"
        
        bpat="cluster=172.17.0.3"
        bcopts=" --cluster=172.17.0.3"
        bself=" --self=0"
        bka=" --ka=172.17.0.3:172.17.0.2"
    fi
    echo "Restarting $master"
    pid=$(docker exec -i $master ps -aef | grep $mpat | xargs | cut -d ' ' -f 2)
    echo "Killing $mpid" >&2
    docker exec -dt $master kill -9 $pid
    docker exec -dt $master ip link del llb0
    docker exec -dt $master nohup /root/loxilb-io/loxilb/loxilb $mcopts $mself $mka > /dev/null &
    pid=$(docker exec -i $master ps -aef | grep $mpat | xargs | cut -d ' ' -f 2)
    echo "New loxilb pid: $pid" >&2

    echo "Restarting $backup"
    pid=$(docker exec -i $backup ps -aef | grep $bpat | xargs | cut -d ' ' -f 2)
    echo "Killing $pid" >&2
    docker exec -dt $backup kill -9 $pid
    docker exec -dt $backup ip link del llb0
    docker exec -dt $backup nohup /root/loxilb-io/loxilb/loxilb $bcopts $bself $bka > /dev/null &
    pid=$(docker exec -i $backup ps -aef | grep $bpat | xargs | cut -d ' ' -f 2)
    echo "New loxilb pid: $pid" >&2

}


