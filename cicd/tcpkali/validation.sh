#!/bin/bash
source ../common.sh
echo SCENARIO-tcpkali
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

conn=400

time=14400 #4hrs

stime=$(( $time + 10 ))
sleep 2
code=0
servIP="20.20.20.1"
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
host=( "l3h1" "l3h2" )
i=0
waitCount=0
while [ $i -le 1 ]
do
j=0
echo "Check connectivity from ${host[i]}"
while [ $j -le 2 ]
do
    res=$($hexec ${host[i]} curl --max-time 10 -s ${ep[j]}:8080)
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
            echo SCENARIO-tcpkali [FAILED]
            sudo killall -9 node 2>&1 > /dev/null
            exit 1
        fi
    fi
    sleep 1
done
i=$(( $i + 1 ))
done

sudo killall -9 node 2>&1 > /dev/null

$hexec l3ep1 tcpkali -l 8080 -T $stime 2>&1> /dev/null &
$hexec l3ep2 tcpkali -l 8080 -T $stime 2>&1> /dev/null &
$hexec l3ep3 tcpkali -l 8080 -T $stime 2>&1> /dev/null &

sleep 2

for k in {0..1}
do
    echo "${host[k]}: Testing Service IP: $servIP - connections: $conn duration: $time secs"
    lcode=0
    $hexec ${host[k]} stdbuf -oL tcpkali -c $conn -T $time -w 8 -m "message" -r 2000 $servIP:2020 2> ${host[k]}.log &
done

#sleep $stime
$hexec llb1 ./pps.sh $time ellb1l3h1 ellb1l3h2

conn1=$( tail -n 2 l3h1.log | xargs | cut -d ' ' -f 11 | cut -d ')' -f 1 )
conn2=$( tail -n 2 l3h2.log | xargs | cut -d ' ' -f 11 | cut -d ')' -f 1 )

if [[ $conn1 == $conn && $conn2 == $conn ]]
then
    echo SCENARIO-tcpkali with ${servIP[k]} [OK]
else
    echo SCENARIO-tcpkali with ${servIP[k]} [FAILED]
    echo "l3h1 tcpkali"
    tail -n 2 l3h1.log
    echo "l3h2 tcpkali"
    tail -n 2 l3h2.log
    code=1
fi

rm l3h1.log l3h2.log

#sudo killall -9 tcpkali 2>&1 > /dev/null
exit $code
