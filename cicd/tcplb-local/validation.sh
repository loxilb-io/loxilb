#!/bin/bash
source ../common.sh
echo SCENARIO-tcplb-local
$hexec llb1 node ../common/tcp_server.js server1 &

sleep 5
code=0
servIP=( "10.10.10.3" )
servArr=( "server1"  )
ep=( "10.10.10.3" )

for k in {0..0}
do
echo "Testing Service IP: ${servIP[k]}"
lcode=0
for j in {0..2}
do
    res=$($hexec llb1 curl --max-time 10 -s ${servIP[k]}:2020)
    echo $res
    if [[ $res != "${servArr[k]}" ]]
    then
        lcode=1
    fi
    sleep 1
done
if [[ $lcode == 0 ]]
then
    echo SCENARIO-tcplb-local with ${servIP[k]} [OK]
else
    echo SCENARIO-tcplb-local with ${servIP[k]} [FAILED]
    code=1
fi
done

sudo killall -9 node 2>&1 > /dev/null
exit $code
