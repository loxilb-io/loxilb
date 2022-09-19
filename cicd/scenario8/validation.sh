#!/bin/bash
source ../common.sh
echo SCENARIO-8
$hexec l3e1 node ./server1.js &
$hexec l3e2 node ./server2.js &
$hexec l3e3 node ./server3.js &

sleep 10
code=0
servArr=( "server1" "server2" "server3" )
ep=( "25.25.25.1" "26.26.26.1" "27.27.27.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    #res=$($hexec h1 curl ${ep[j]}:8080)
    res=`$hexec h1 curl --max-time 10 -s ${ep[j]}:8080`
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
            echo SCENARIO-8 [FAILED]
            exit 1
        fi
    fi
    sleep 1
done

for k in {1..2}
do
for i in {1..2}
do
for j in {0..2}
do
    #$hexec h$k ping ${ep[j]} -f -c 5 -W 1;
    res=`$hexec h$k curl --max-time 10 -s 88.88.88.88:2020`
    #echo -e $res
    if [[ $res != "${servArr[j]}" ]]
    then
        echo -e "Expected ${servArr[j]}, Received : $res"
        if [[ "$res" != *"server"* ]];
        then
            echo "llb1 ct"
            $dexec llb1 loxicmd get ct
            echo "llb2 ct"
            $dexec llb2 loxicmd get ct
            echo "llb2 ip neigh"
            $dexec llb2 ip neigh
        fi
        code=1
    fi
    sleep 1
done
done
done
if [[ $code == 0 ]]
then
    echo SCENARIO-8 [OK]
else
    echo SCENARIO-8 [FAILED]
fi
exit $code

