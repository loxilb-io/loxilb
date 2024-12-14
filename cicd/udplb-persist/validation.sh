#!/bin/bash
source ../common.sh

echo SCENARIO-udplb-persist
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
            echo SCENARIO-udplb-persist [FAILED]
            sudo pkill udp_server 2>&1 > /dev/null
            exit 1
        fi

    fi
    sleep 1
done

for i in {1..3}
do
pres=""
echo -e "Persist Test for host$i"
for j in {0..2}
do
    res=$($hexec l3h${i} timeout 1 ../common/udp_client 20.20.20.1 2020)
    if [[ $pres == "" ]]; then
      pres=$res
    else
      if [[ $res != $pres ]]; then
        echo -e "Expected $pres Received : $res for host$i"
        code=1
      fi
    fi
    echo -e $res
    sleep 1
done
done
if [[ $code == 0 ]]
then
    echo SCENARIO-udplb-persist [OK]
else
    echo SCENARIO-udplb-persist [FAILED]
fi
sudo pkill udp_server 2>&1 > /dev/null
