#!/bin/bash
source ../common.sh

$hexec l3ep1 node ./server1.js &
$hexec l3ep2 node ./server2.js &
$hexec l3ep3 node ./server3.js &

$hexec llb1 loxicmd -p 11112 create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1
sleep 4
code=0
servArr=( "server1" "server2" "server3" ) 
for i in {1..5}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl -s 20.20.20.1:2020)
    echo $res
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
done
done
if [[ $code == 0 ]]
then
    echo [OK]
else
    echo [FAILED]
fi
exit $code

