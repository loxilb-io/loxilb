#!/bin/bash
source ../common.sh
echo "SCENARIO nat64tcp"
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &

sleep 5
code=0
servIP=( "3ffe::2" "2001::1" )
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
            echo SCENARIO-nat64tcp [FAILED]
            sudo pkill node
            exit 1
        fi
    fi
    sleep 1
done

res=$($hexec l3h1 curl -s --max-time 20 '[2001::1]:2020')
res=$($hexec l3h1 curl -s --max-time 20 '[3ffe::2]:2020')
sleep 4

nid=0
for k in {0..1}
do
echo "Testing Service IP: ${servIP[k]}"
lcode=0
for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 curl -s -j -6 --max-time 10 "[${servIP[k]}]:2020")
    echo $res
    ids=`echo "${res//[!0-9]/}"`
    if [[ $res == *"server"* ]]; then
      ids=`echo "${res//[!0-9]/}"`
      if [[ $nid == 0 ]];then
        nid=$((($ids + 1)%4))
        if [[ $nid == 0 ]];then
          nid=1
        fi
      elif [[ $nid != $((ids)) ]]; then
        echo "Expected server$nid got server$((ids))"
        lcode=1
      fi
      nid=$((($ids + 1)%4))
      if [[ $nid == 0 ]];then
        nid=1
      fi
    else
      lcode=1
    fi
    sleep 1
done
done
if [[ $lcode == 0 ]]
then
    echo nat64tcp with ${servIP[k]} [OK]
else
    echo nat64tcp with ${servIP[k]} [FAILED]
    code=1 
fi

done
sudo pkill -9 node
if [[ $code == 0 ]]
then
    echo nat64tcp [OK]
else
    echo nat64tcp [FAILED]
fi
exit $code

