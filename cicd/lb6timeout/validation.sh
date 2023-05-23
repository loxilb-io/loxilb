#!/bin/bash
source ../common.sh
echo LB6-TIMEOUT
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "4ffe::1" "5ffe::1" "6ffe::1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    svr=${ep[j]}
    res=$($hexec l3h1 curl -s -j -6 --max-time 10 [${svr}]:8080)
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
            echo LB6-TIMEOUT[FAILED]
            sudo pkill node
            exit 1
        fi
    fi
    sleep 2
done

SERVICE="nc"
$hexec l3ep1 iperf -s -p 8080 -V -B 4ffe::1 >> /dev/null 2>&1 &
$hexec l3ep2 iperf -s -p 8080 -V -B 5ffe::1 >> /dev/null 2>&1 &
$hexec l3ep3 iperf -s -p 8080 -V -B 6ffe::! >> /dev/null 2>&1 &
sleep 30
$hexec l3h1 nohup nc -6 2001::1 2020 >> /dev/null 2>&1 &
ncpid=$!

sleep 10
if pgrep -x "$SERVICE" >/dev/null
then
    echo $SERVICE is UP
else
    echo $SERVICE is DOWN
    exit 1
fi

sleep 60

# For this scenario, loxilb will send reset after configured inactivity of 30s

code=0
if pgrep -x "$SERVICE" >/dev/null
then
    echo LB6-TIMEOUT [FAILED]
    code=1
else
    echo LB6-TIMEOUT [OK]
    code=0
fi
sudo killall -9 iperf >> /dev/null 2>&1
sudo kill -9 $ncpid >> /dev/null 2>&1
sudo killall -9 nc >> /dev/null 2>&1
sudo killall -9 node >> /dev/null 2>&1
sudo rm -f nohup.out
exit $code
