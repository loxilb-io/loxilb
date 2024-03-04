#!/bin/bash
source ../common.sh
echo SCENARIO-tcplb-lc
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
servIP=( "20.20.20.1" )
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 curl --max-time 10 -s ${ep[j]}:8080)
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
            echo SCENARIO-tcplb-lc [FAILED]
            sudo killall -9 node 2>&1 > /dev/null
            exit 1
        fi
    fi
    sleep 1
done

echo "Testing Service IP: ${servIP[0]}"
lcode=0
for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl --max-time 10 -s ${servIP[0]}:2020)
    echo $res
    if [[ $res != "${servArr[0]}" ]]
    then
        lcode=1
    fi
    sleep 1
done
done

$hexec l3h1 nohup nc -d ${servIP[0]} 2020  &
sleep 5

echo "Testing Service IP: ${servIP[0]}"
lcode=0
for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl --max-time 10 -s ${servIP[0]}:2020)
    echo $res
    if [[ $res != "server2" ]]
    then
        lcode=1
    fi
    sleep 1
done
done

if [[ $lcode == 0 ]]
then
    echo SCENARIO-tcplb with least-connection [OK]
else
    echo SCENARIO-tcplb with least-connection [FAILED]
    code=1
fi

sudo killall -9 node 2>&1 > /dev/null
sudo killall -9 nc 2>&1 > /dev/null
rm -f nohup.out

exit $code
