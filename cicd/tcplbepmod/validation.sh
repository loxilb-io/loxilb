#!/bin/bash
source ../common.sh
echo SCENARIO-tcplbepmod
$hexec l3ep1 socat -v -T0.05 tcp-l:8080,reuseaddr,fork system:"echo 'server1'; cat" >/dev/null 2>&1 &
$hexec l3ep2 socat -v -T0.05 tcp-l:8080,reuseaddr,fork system:"echo 'server2'; cat" >/dev/null 2>&1 &
$hexec l3ep3 socat -v -T0.05 tcp-l:8080,reuseaddr,fork system:"echo 'server3'; cat" >/dev/null 2>&1 &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 socat -T10 - TCP:${ep[j]}:8080)
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
            echo SCENARIO-tcplbepmod [FAILED]
            exit 1
        fi
    fi
    sleep 1
done


echo "Expecting server1"

for j in {0..3}
do
    res=$($hexec l3h1 socat -T10 - TCP:20.20.20.1:2020)
    echo $res
    if [[ $res != "server1" ]]
    then
        code=1
    fi
    sleep 1
done

$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1
sleep 5
echo "Expecting server1, server2"

for j in {0..3}
do
    res=$($hexec l3h1 socat -T10 - TCP:20.20.20.1:2020)
    echo $res
    if [[ $res != "server1" ]] && [[ $res != "server2" ]]
    then
        code=1
    fi
    sleep 1
done


$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1
sleep 5
echo "Expecting server1"
for j in {0..3}
do
    res=$($hexec l3h1 socat -T10 - TCP:20.20.20.1:2020)
    echo $res
    if [[ $res != "server1" ]]
    then
        code=1
    fi
    sleep 1
done

$dexec llb1 loxicmd create lb 20.20.20.1 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1
sleep 5
echo "Expecting server1, server2"

for j in {0..3}
do
    res=$($hexec l3h1 socat -T10 - TCP:20.20.20.1:2020)
    echo $res
    if [[ $res != "server1" ]] && [[ $res != "server2" ]]
    then
        code=1
    fi
    sleep 1
done

if [[ $code == 0 ]]
then
    echo SCENARIO-tcplbepmod [OK]
else
    echo SCENARIO-tcplbepmod [FAILED]
fi
$hexec l3ep1 killall  -9 socat > /dev/null 2>&1
$hexec l3ep2 killall  -9 socat > /dev/null 2>&1
$hexec l3ep3 killall  -9 socat > /dev/null 2>&1
exit $code
