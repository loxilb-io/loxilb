#!/bin/bash
source ../common.sh

$hexec n1p1 node ./server1.js &
$hexec n2p1 node ./server2.js &
$hexec n3p1 node ./server3.js &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "33.33.33.1" "34.34.34.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec n1p1 curl -s ${ep[j]}:8080)
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
            exit 1
        fi
    fi
    sleep 1
done

for i in {1..4}
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

