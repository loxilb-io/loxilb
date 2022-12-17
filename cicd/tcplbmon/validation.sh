#!/bin/bash
source ../common.sh
echo SCENARIO-tcplbmon
$hexec l3ep1 node ./server1.js &
$hexec l3ep2 node ./server2.js &
$hexec l3ep3 node ./server3.js &

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
            echo SCENARIO-tcplbmon [FAILED]
            exit 1
        fi
    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl --max-time 10 -s 20.20.20.1:2020)
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
    echo SCENARIO-tcplbmon p1 [OK]
else
    echo SCENARIO-tcplbmon p1 [FAILED]
    $hexec l3ep1 killall -9 node > /dev/null 2>&1
    $hexec l3ep2 killall -9 node > /dev/null 2>&1
    $hexec l3ep3 killall -9 node > /dev/null 2>&1
    exit $code
fi

$hexec l3ep1 ip addr del 31.31.31.1/24 dev el3ep1llb1
sleep 45

for j in {0..5}
do
    res=$($hexec l3h1 curl --max-time 10 -s 20.20.20.1:2020)
    echo $res
    if [[ $res == "server1" ]] && [[ $res != "" ]] 
    then
        code=1
    fi
    sleep 1
done
if [[ $code == 0 ]]
then
    echo SCENARIO-tcplbmon [OK]
else
    echo SCENARIO-tcplbmon [FAILED]
fi
$hexec l3ep1 killall -9 node > /dev/null 2>&1
$hexec l3ep2 killall -9 node > /dev/null 2>&1
$hexec l3ep3 killall -9 node > /dev/null 2>&1
exit $code

