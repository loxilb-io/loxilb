#!/bin/bash
source ../common.sh
echo SCENARIO-sctplbmon
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
$hexec l3ep1 ../common/sctp_server ${ep[0]} 8080 server1 >/dev/null 2>&1 &
$hexec l3ep2 ../common/sctp_server ${ep[1]} 8080 server2 >/dev/null 2>&1 &
$hexec l3ep3 ../common/sctp_server ${ep[2]} 8080 server3 >/dev/null 2>&1 &

j=0
waitCount=0
sleep 15

while [ $j -le 2 ]
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_client 10.10.10.1 0 ${ep[j]} 8080)
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
            sudo killall -9 sctp_server 2>&1 > /dev/null
            echo SCENARIO-sctplbmon [FAILED]
            exit 1
        fi
    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_client 10.10.10.1 0 20.20.20.1 2020)
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
    echo SCENARIO-sctplbmon p1 [OK]
else
    echo SCENARIO-sctplbmon p1 [FAILED]
    sudo killall -9 sctp_server 2>&1 > /dev/null
    exit $code
fi

$hexec l3ep1 ip addr del 31.31.31.1/24 dev el3ep1llb1
echo "Waiting 140s...."
sleep 140
$dexec llb1 loxicmd get ep

for j in {0..5}
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_client 10.10.10.1 0 20.20.20.1 2020)
    if [[ $res == "server1" ]] && [[ "empty"$res == "empty" ]] 
    then
        code=1
    fi
    sleep 1
done
if [[ $code == 0 ]]
then
    echo SCENARIO-sctplbmon p2 [OK]
else
    echo SCENARIO-sctplbmon p2 [FAILED]
    sudo killall -9 node 2>&1 > /dev/null
    exit $code
fi

$hexec l3ep1 ip addr add 31.31.31.1/24 dev el3ep1llb1
$hexec l3ep1 ip route add default via 31.31.31.254
sudo killall -9 sctp_server 2>&1 > /dev/null
$hexec l3ep1 ../common/sctp_server ${ep[0]} 8080 server1 >/dev/null 2>&1 &
$hexec l3ep2 ../common/sctp_server ${ep[1]} 8080 server2 >/dev/null 2>&1 &
$hexec l3ep3 ../common/sctp_server ${ep[2]} 8080 server3 >/dev/null 2>&1 &
sleep 30
$dexec llb1 loxicmd get ep

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_client 10.10.10.1 0 20.20.20.1 2020)
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
    echo SCENARIO-sctplbmon p3 [OK]
else
    echo SCENARIO-sctplbmon p3 [FAILED]
fi

sudo killall -9 sctp_server 2>&1 > /dev/null
exit $code
