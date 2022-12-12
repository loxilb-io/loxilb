#!/bin/bash
source ../common.sh
echo SCENARIO-12
$hexec l3ep1 node ./server1.js &
$hexec l3ep2 node ./server2.js &
$hexec l3ep3 node ./server3.js &

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
            echo SCENARIO-12[FAILED]
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
    exit 1
fi

sleep 100

# For this scenario, loxilb will send reset after configured inactivity of 30s

code=0
if pgrep -x "$SERVICE" >/dev/null
then
    echo SCENARIO-12 [FAILED]
    code=1
else
    echo SCENARIO-12 [OK]
    code=0
fi
sudo killall -9 iperf >> /dev/null 2>&1
sudo kill -9 $ncpid >> /dev/null 2>&1
sudo killall -9 nc >> /dev/null 2>&1
sudo rm -f nohup.out
exit $code
