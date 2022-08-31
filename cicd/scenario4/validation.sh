#!/bin/bash
source ../common.sh

$hexec n1p1 node ./server1.js &
$hexec n2p1 node ./server2.js &
$hexec n3p1 node ./server3.js &

sleep 2
code=0
servArr=( "server1" "server2" "server3" ) 
for i in {1..5}
do
for j in {0..2}
do
    res=$($hexec n1p1 curl -s 20.20.20.1:2020)
    echo -e $res
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

