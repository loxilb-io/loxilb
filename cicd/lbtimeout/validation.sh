#!/bin/bash
source ../common.sh
echo LB-TIMEOUT
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
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
            echo LB-TIMEOUT[FAILED]
            sudo pkill node
            exit 1
        fi
    fi
    sleep 2
done

SERVICE="nc"
$hexec l3ep1 iperf -s -p 8080 >> /dev/null 2>&1 &
$hexec l3ep2 iperf -s -p 8080 >> /dev/null 2>&1 &
$hexec l3ep3 iperf -s -p 8080 >> /dev/null 2>&1 &
sleep 30
$hexec l3h1 nohup nc -d 20.20.20.1 2020 >> /dev/null 2>&1 &
ncpid=$!

sleep 10
if pgrep -x "$SERVICE" >/dev/null
then
    echo $SERVICE is UP
else
    echo $SERVICE is DOWN
    sudo pkill node
    exit 1
fi

sleep 65

# For this scenario, loxilb will send reset after configured inactivity of 30s

code=0
if pgrep -x "$SERVICE" >/dev/null
then
    echo LB-TIMEOUT [FAILED]
    code=1
else
    echo LB-TIMEOUT [OK]
    code=0
fi
sudo killall -9 iperf >> /dev/null 2>&1
sudo kill -9 $ncpid >> /dev/null 2>&1
sudo killall -9 nc >> /dev/null 2>&1
sudo killall -9 node >> /dev/null 2>&1
sudo rm -f nohup.out
exit $code
