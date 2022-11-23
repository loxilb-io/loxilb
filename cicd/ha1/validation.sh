#!/bin/bash
source ../common.sh
echo SCENARIO-12

function myfunc() {
$hexec ep1 node ./server1.js &
$hexec ep2 node ./server2.js &
$hexec ep3 node ./server3.js &

sleep 5

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
            echo SCENARIO-12 [FAILED] >&2
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
echo $code
}

status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/hastate/all' -H 'accept: application/json' | jq -r '.schema.state')
status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/hastate/all' -H 'accept: application/json' | jq -r '.schema.state')
echo SCENARIO-12 HA state llb1-$status1 llb2-$status2

if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
then
    master="llb1"
    backup="llb2"
elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; 
then
    master="llb2"
    backup="llb1"
else
    echo SCENARIO-12 HA state llb1-$status1 llb2-$status2 [FAILED]
fi
echo "Master:$master Backup:$backup"

code=$(myfunc)
if [[ $code == 0 ]]
then
    echo SCENARIO-12 Phase-1 [OK]
    $dexec $master systemctl restart keepalived.service
else
    echo SCENARIO-12 Phase-1 [FAILED]
fi

sleep 1

status1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/hastate/all' -H 'accept: application/json' | jq -r '.schema.state')
status2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/hastate/all' -H 'accept: application/json' | jq -r '.schema.state')
echo SCENARIO-12 HA state $master-$status1 $backup-$status2

if [[ $status2 == "MASTER" && $status1 == "BACKUP" ]];
then
    echo SCENARIO-12 HA [SUCCESS]
else
    echo SCENARIO-12 HA [FAILED]
    exit 1
fi

code=$(myfunc)
if [[ $code == 0 ]]
then
    echo SCENARIO-12 [OK]
else
    echo SCENARIO-12 [FAILED]
fi

exit $code
