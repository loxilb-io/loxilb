#!/bin/bash
source ../common.sh
echo SCENARIO-masquerade
$hexec l3h1 node ../common/tcp_server.js server1 &

sleep 15
code=0
servIP=( "10.10.10.1" )
servArr=( "server1" )
clientArr=( "l3ep1" "l3ep2" "l3ep3" )
j=0
waitCount=0

echo "Testing Service IP: ${servIP[0]}"
lcode=0
for i in {0..2}
do
for j in {0..2}
do
    res=$($hexec ${clientArr[i]} curl --max-time 10 -s ${servIP[0]}:8080)
    echo $res
    if [[ $res != "${servArr[0]}" ]]
    then
        lcode=1
    fi
    sleep 1
done
done

if [[ $lcode == 0 ]]
then
    echo SCENARIO-masquerade with ${servIP[0]} [OK]
else
    echo SCENARIO-masquerade with ${servIP[0]} [FAILED]
    code=1
fi

sudo killall -9 node 2>&1 > /dev/null
exit $code
