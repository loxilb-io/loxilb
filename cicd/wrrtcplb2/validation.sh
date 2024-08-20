#!/bin/bash
source ../common.sh
echo SCENARIO-wrrtcplb2
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
            echo SCENARIO-wrrtcplb2 [FAILED]
            sudo pkill node
            exit 1
        fi
    fi
    sleep 1
done

respArr=( "server1" "server1" "server1"
          "server1" "server1" "server1"
          "server1" "server1" "server1"
          "server1" "server1" "server1"
          "server2" "server2" "server2"
          "server2" "server2" "server2"
          "server2" "server2" "server2"
          "server2" "server2" "server2"
          "server3" "server3" "server3"
          "server3" "server3" "server3"
          "server1" "server1"
        )

for i in {0..31}
do
    res=$($hexec l3h1 curl --max-time 10 -s 20.20.20.1:2020)
    echo $i:$res
    if [[ $res != "${respArr[i]}" ]]
    then
        echo "expected ${respArr[i]} rcvd $res"
        code=1
    fi
    sleep 1
done
sudo killall -9 node 2>&1 > /dev/null
if [[ $code == 0 ]]
then
    echo SCENARIO-wrrtcplb2 [OK]
else
    echo SCENARIO-wrrtcplb2 [FAILED]
fi
exit $code
