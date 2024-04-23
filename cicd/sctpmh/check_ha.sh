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
            echo "KA llb1-$status1, llb2-$status2 [NOK]" >&2
            exit 1;
        fi
        sleep 5
    fi
done
}

function checkSync() {
count=1
sync=0
while [[ $count -le 5 ]] ; do
echo -e "\nStatus at MASTER:$master\n" >&2
$dexec $master loxicmd get ct | grep est >&2

echo -e "\nStatus at BACKUP:$backup\n" >&2
$dexec $backup loxicmd get ct | grep est >&2

nres1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)
nres2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)

if [[ $nres1 == 0 ]]; then
    echo -e "No active connections in Master:$master. Exiting!" >&2
    return 0
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
