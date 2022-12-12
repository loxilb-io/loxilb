#!/bin/bash
source ../common.sh
echo "SCENARIO nat66tcp"
$hexec l3ep1 node ./server1.js &
$hexec l3ep2 node ./server2.js &
$hexec l3ep3 node ./server3.js &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "4ffe::1" "5ffe::1" "6ffe::1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    echo HELLLO
    #res=$($hexec l3h1 curl -g -6 --max-time 10 ${ep[j]}:2020)
    res=$($hexec l3h1 curl --max-time 10 -s ${ep[j]}:8080)
    echo $res
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
            echo nat66tcp [FAILED]
            exit 1
        fi
    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl -j -6 --max-time 10 '[2001::1]:2020')
    echo $res
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
    sleep 1
done
done
if [[ $code == 0 ]]
then
    echo nat66tcp [OK]
else
    echo nat66tcp [FAILED]
fi
exit $code

