#!/bin/bash
source ../common.sh
echo SCENARIO-e2ehttps-tcplb
$hexec l3ep1 node ../common/tcp_https_server.js server1 loxilb.io &
$hexec l3ep2 node ../common/tcp_https_server.js server2 loxilb.io &
$hexec l3ep3 node ../common/tcp_https_server.js server3 loxilb.io &

sleep 5
code=0
servIP=( "10.10.10.254" )
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0

for k in {0..0}
do
echo "Testing Service IP: ${servIP[k]}"
lcode=0
for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl --max-time 10 -H "Application/json" -H "Content-type: application/json" -H "HOST: loxilb.io" --insecure -s https://${servIP[k]}:2020)
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
    echo SCENARIO-e2ehttps-tcplb with ${servIP[k]} [OK]
else
    echo SCENARIO-e2ehttps-tcplb with ${servIP[k]} [FAILED]
    code=1
fi
done

sudo killall -9 node 2>&1 > /dev/null
exit $code
