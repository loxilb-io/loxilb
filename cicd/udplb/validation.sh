#!/bin/bash
source ../common.sh

echo SCENARIO-udplb
$hexec l3ep1 ../common/udp_server 8080 server1 &
$hexec l3ep2 ../common/udp_server 8080 server2 &
$hexec l3ep3 ../common/udp_server 8080 server3 &

sleep 5
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 timeout 1 ../common/udp_client ${ep[j]} 8080)
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
            echo SCENARIO-udplb [FAILED]
            sudo pkill udp_server 2>&1 > /dev/null
            exit 1
        fi

    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 1 ../common/udp_client 20.20.20.1 2020)
    #res=$($hexec l3h1 timeout 1 ./client ${ep[j]} 8080)
    echo -e $res
    if [[ $res != "${servArr[j]}" ]]
    then
        echo -e "Expected ${servArr[j]}, Received : $res"
        if [[ "$res" != *"server"* ]];
        then
            echo "llb1 ct"
            $dexec llb1 loxicmd get ct
            echo "llb1 ip neigh"
            $dexec llb1 ip neigh
        fi
        code=1
    fi
    sleep 1
done
done
if [[ $code == 0 ]]
then
    echo SCENARIO-udplb [OK]
else
    echo SCENARIO-udplb [FAILED]
fi
sudo pkill udp_server 2>&1 > /dev/null

$hexec l3ep1 ./udp_jumbo_app server 8080 &
$hexec l3ep2 ./udp_jumbo_app server 8080 &
$hexec l3ep3 ./udp_jumbo_app server 8080 &

sleep 5

for j in {0..2}
do
    res=$($hexec l3h1 timeout 3 ./udp_jumbo_app client 20.20.20.1:2020)
    echo -e "FRAG TEST RUN$j:$res"
    if [[ $res != "OK" ]]
    then
        echo -e "Run $j: Expected OK, Received : $res"
        code=1
    fi
    sleep 1
done

sudo pkill udp_jumbo_app 2>&1 > /dev/null

exit $code
