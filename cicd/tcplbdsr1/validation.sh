#!/bin/bash
source ../common.sh
echo SCENARIO-tcplbdsr1
$hexec l3ep1 socat -v -T0.05 tcp-l:2020,reuseaddr,fork system:"echo 'server1'; cat" >/dev/null 2>&1 &
$hexec l3ep2 socat -v -T0.05 tcp-l:2020,reuseaddr,fork system:"echo 'server2'; cat" >/dev/null 2>&1 &
$hexec l3ep3 socat -v -T0.05 tcp-l:2020,reuseaddr,fork system:"echo 'server3'; cat" >/dev/null 2>&1 &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 socat -T10 - TCP:${ep[j]}:2020)
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
            echo SCENARIO-tcplbdsr [FAILED]
            exit 1
        fi
    fi
    sleep 1
done

for j in {0..2}
do
    res=$($hexec l3h1 socat -T10 - TCP:20.20.20.1:2020,sp=55001,reuseaddr)
    echo $res
    if [[ $exp == "" ]]
    then
      exp=$res
    fi
    if [[ $exp != $res ]]
    then
      code=1
    fi
    sleep 1
done
if [[ $code == 0 ]]
then
    echo SCENARIO-tcplbdsr1 [OK]
else
    echo SCENARIO-tcplbdsr1 [FAILED]
fi
$hexec l3ep1 killall  -9 socat > /dev/null 2>&1
$hexec l3ep2 killall  -9 socat > /dev/null 2>&1
$hexec l3ep3 killall  -9 socat > /dev/null 2>&1
exit $code
