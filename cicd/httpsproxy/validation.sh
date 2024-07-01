#!/bin/bash
source ../common.sh
echo SCENARIO-https-tcplb
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
servIP=( "10.10.10.254" )
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
            echo SCENARIO-tcplb [FAILED]
            sudo killall -9 node 2>&1 > /dev/null
            exit 1
        fi
    fi
    sleep 1
done

for k in {0..0}
do
echo "Testing Service IP: ${servIP[k]}"
lcode=0
for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl --max-time 10 --insecure -s https://${servIP[k]}:2020)
    echo $res
    if [[ $res != "${servArr[j]}" ]]
    then
        lcode=1
    fi
    sleep 1
done
done
if [[ $lcode == 0 ]]
then
    echo SCENARIO-https-tcplb with ${servIP[k]} [OK]
else
    echo SCENARIO-https-tcplb with ${servIP[k]} [FAILED]
    code=1
fi
done

sudo killall -9 node 2>&1 > /dev/null
exit $code
