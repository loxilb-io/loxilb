#!/bin/bash
source ../common.sh
echo HA-1

function myfunc() {
$hexec ep1 node ../common/tcp_server.js server1 &
$hexec ep2 node ../common/tcp_server.js server2 &
$hexec ep3 node ../common/tcp_server.js server3 &

sleep 30

local code=0
servArr=( "server1" "server2" "server3" )
ep=( "11.11.11.3" "11.11.11.4" "11.11.11.5" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec user curl --max-time 10 -s ${ep[j]}:8080)
    #echo $res
    if [[ $res == "${servArr[j]}" ]]
    then
        echo "$res UP" >&2
        j=$(( $j + 1 ))
    else
        echo "Waiting for ${servArr[j]}(${ep[j]})" >&2
        waitCount=$(( $waitCount + 1 ))
        if [[ $waitCount == 11 ]];
        then
            echo "All Servers are not UP" >&2
            echo HA-1 [FAILED] >&2
            sudo pkill node
            exit 1
        fi
    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec user curl --max-time 10 -s 20.20.20.1:2020)
    echo -e $res >&2
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
    sleep 1
done
done
sudo pkill node
echo $code
}

status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
echo HA-1 HA state llb1-$status1 llb2-$status2

if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
then
    master="llb1"
    backup="llb2"
elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; 
then
    master="llb2"
    backup="llb1"
else
    echo HA-1 HA state llb1-$status1 llb2-$status2 [FAILED]
fi
echo "Master:$master Backup:$backup"

code=$(myfunc)
if [[ $code == 0 ]]
then
    echo HA-1 Phase-1 [OK]
    docker restart ka_$master
else
    echo HA-1 Phase-1 [FAILED]
fi

sleep 1

status1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
status2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
echo HA-1 HA state $master-$status1 $backup-$status2

if [[ $status2 == "MASTER" && $status1 == "BACKUP" ]];
then
    echo HA-1 HA [SUCCESS]
else
    echo HA-1 HA [FAILED]
    exit 1
fi

code=$(myfunc)
if [[ $code == 0 ]]
then
    echo HA-1 [OK]
else
    echo HA-1 [FAILED]
fi

exit $code
