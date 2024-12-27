#!/bin/bash
source ../common.sh
echo ONEARM-L2
$hexec l2ep1 node ../common/tcp_server.js server1 &
$hexec l2ep2 node ../common/tcp_server.js server2 &
$hexec l2ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "100.100.100.2" "100.100.100.3" "100.100.100.4" )
j=3
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l2h1 curl --max-time 10 -s ${ep[j]}:8080)
    #echo $res
    if [[ $res == "${servArr[j]}" ]]
    then
        echo "$res UP"
        j=$(( $j + 1 ))
    else
        echo "Waiting for ${servArr[j]}(${ep[j]})"
        waitCount=$(( $waitCount + 1 ))
        if [[ $waitCount == 10 ]];
        then
            echo "All Servers are not UP"
            echo ONEARM-L2 [FAILED]
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
    res=$($hexec l2h1 curl --max-time 10 -s 20.20.20.1:2020)
    echo $res
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
    sleep 1
done
done
if [[ $code == 0 ]]
then
    echo ONEARM-L2 [OK]
else
    $dexec llb1 loxicmd get ct
    $dexec llb1 loxicmd get lb -o wide
    $dexec llb1 loxicmd get ep -o wide
    $dexec llb1 loxicmd get port
    $dexec llb1 ip neigh
    echo ONEARM-L2 [FAILED]
fi
sudo pkill node
exit $code
