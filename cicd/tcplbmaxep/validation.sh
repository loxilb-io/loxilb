#!/bin/bash
source ../common.sh
echo SCENARIO-tcplb-maxep
$hexec l3ep1 node ../common/tcp_server.js server1 &
$hexec l3ep2 node ../common/tcp_server.js server2 &
$hexec l3ep3 node ../common/tcp_server.js server3 &
$hexec l3ep4 node ../common/tcp_server.js server4 &

sleep 5
code=0
servIP=( "20.20.20.1" )
servArr=(
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
          "server1" "server2" "server3" "server4" 
        )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" "34.34.34.1" )
j=0
waitCount=0
while [ $j -le 3 ]
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
            echo SCENARIO-tcplb-maxep [FAILED]
            sudo killall -9 node 2>&1 > /dev/null
            exit 1
        fi
    fi
    sleep 1
done

echo "Testing Service IP: ${servIP[0]}"
lcode=0
for i in {0..31}
do
  res=$($hexec l3h1 curl --max-time 10 -s ${servIP[0]}:2020)
  echo $res
  if [[ $res != "${servArr[i]}" ]]
  then
    lcode=1
  fi
  sleep 1

  if [[ $lcode == 0 ]]
  then
    echo SCENARIO-tcplb-maxep with ep${i+1} 35.${i}.${j}.1 [OK]
  else
    echo SCENARIO-tcplb-maxep with ep${i+1} 35.${i}.${j}.1 [FAILED]
    code=1
  fi
done

if [[ $code == 0 ]]; then
  echo SCENARIO-tcplb-maxep [OK]
else
  echo SCENARIO-tcplb-maxep [FAILED]
fi

sudo killall -9 node 2>&1 > /dev/null
exit $code
