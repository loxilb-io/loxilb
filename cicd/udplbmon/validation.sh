#!/bin/bash
source ../common.sh
echo SCENARIO-udplbmon
$hexec l3ep1 ../common/udp_server 8080 server1 &
$hexec l3ep2 ../common/udp_server 8080 server2 &
$hexec l3ep3 ../common/udp_server 8080 server3 &

sleep 15
ps -ef | grep udp_server
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 timeout 1 ../common/udp_client ${ep[j]} 8080)
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
            sudo pkill udp_server 2>&1 >> /dev/null
            echo SCENARIO-udplbmon [FAILED]
            exit 1
        fi
    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 1 ../common/udp_client 20.20.20.1 2020)
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
    echo SCENARIO-udplbmon p1 [OK]
else
    echo SCENARIO-udplbmon p1 [FAILED]
    sudo pkill udp_server 2>&1 >> /dev/null
    exit $code
fi

sudo pkill udp_server 2>&1 >> /dev/null
sleep 1
#$hexec l3ep1 ../common/udp_server 8080 server1 &
$hexec l3ep2 ../common/udp_server 8080 server2 &
$hexec l3ep3 ../common/udp_server 8080 server3 &
echo "Waiting 140s...."
sleep 140
$dexec llb1 loxicmd get ep

for j in {0..5}
do
    res=$($hexec l3h1 timeout 1 ../common/udp_client 20.20.20.1 2020)
    echo $res
    if [[ $res == "server1" ]] && [[ "empty"$res == "empty" ]] 
    then
        code=1
    fi
    sleep 1
done
if [[ $code == 0 ]]
then
    echo SCENARIO-udplbmon p2 [OK]
else
    echo SCENARIO-udplbmon p2 [FAILED]
    sudo killall -9 node 2>&1 > /dev/null
    exit $code
fi

sudo pkill udp_server 2>&1 >> /dev/null
sleep 1
$hexec l3ep1 ../common/udp_server 8080 server1 &
$hexec l3ep2 ../common/udp_server 8080 server2 &
$hexec l3ep3 ../common/udp_server 8080 server3 &
echo "Waiting 30s...."
sleep 30
$dexec llb1 loxicmd get ep

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 1 ../common/udp_client 20.20.20.1 2020)
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
    echo SCENARIO-udplbmon p3 [OK]
else
    echo SCENARIO-udplbmon p3 [FAILED]
fi

sudo pkill udp_server 2>&1 >> /dev/null
exit $code
