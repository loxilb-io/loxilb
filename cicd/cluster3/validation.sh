#!/bin/bash
source ../common.sh
echo CLUSTER-3

declare -A llbIp
llbIp["llb1"]="11.11.11.1"
llbIp["llb2"]="11.11.11.2"

function tcp_validate() {
  $hexec ep1 node ./server1.js &
  $hexec ep2 node ./server2.js &
  $hexec ep3 node ./server3.js &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )

  echo "Master-$1(${llbIp[$1]})" >&2
  $hexec r1 ip route list match 20.20.20.1 | grep -w ${llbIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route list match 20.20.20.1 >&2
    echo "BGP Service Route [NOK]" >&2
    return 1
  fi 

  for i in {1..5}
  do
  for j in {0..2}
  do
    res=$($hexec user curl --max-time 10 -s 20.20.20.1:2020)
    echo -e $res >&2
    if [[ $res != "${servArr[j]}" ]]
    then
        echo "Expected : "${servArr[j]}", Received : $res" >&2
        code=1
    fi
    sleep 1
  done
  done
  echo $code
}

function sctp_validate() {
  $hexec ep1 ./server server1 &
  $hexec ep2 ./server server2 &
  $hexec ep3 ./server server3 &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )

  echo "Master-$1(${llbIp[$1]})" >&2
  $hexec r1 ip route list match 20.20.20.1 | grep -w ${llbIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route list match 20.20.20.1 >&2
    echo "BGP Service Route [NOK]" >&2
    return 1
  fi 

  for i in {1..5}
  do
  for j in {0..2}
  do
    res=$($hexec user ./client 20.20.20.1 2020)
    echo -e $res >&2
    if [[ $res != "${servArr[j]}" ]]
    then
        echo "Expected : "${servArr[j]}", Received : $res" 2>&1
        code=1
    fi
    sleep 1
  done
  done
  echo $code
}

count=0
while : ; do
  $dexec llb1 gobgp neigh | grep "Estab" 2>&1 >> /dev/null
  if [[ $? -eq 0 ]]; then
    echo "llb1 BGP connection [OK]"
    break;
  fi
  sleep 0.2
  count=$(( $count + 1 ))
  if [[ $count -ge 2000 ]]; then
    echo "llb1 BGP connection [NOK]"
    exit 1;
  fi
done

count=0
while : ; do
  $dexec llb2 gobgp neigh | grep "Estab" >> /dev/null
  if [[ $? -eq 0 ]]; then
    echo "llb2 BGP connection [OK]"
    break;
  fi
  sleep 0.2
  count=$(( $count + 1 ))
  if [[ $count -ge 2000 ]]; then
    echo "$backup BGP connection [NOK]"
    exit 1;
  fi
done

rnh=$($hexec r1 ip route list match 20.20.20.1 | grep "proto zebra" | cut -d ' ' -f 3)

if [[ $rnh == "11.11.11.1" ]];
then
    master="llb1"
    backup="llb2"
elif [[ $rnh == "11.11.11.2" ]]; 
then
    master="llb2"
    backup="llb1"
else
    echo CLUSTER-3 Service Route not advertised to External Router [FAILED]
    exit 1
fi

echo "Master:$master Backup:$backup"


echo "CLUSTER-3 TCP Start"
code=0
code=$(tcp_validate $master)
if [[ $code == 0 ]]
then
    echo CLUSTER-3 TCP Phase-1 [OK]
else
    echo CLUSTER-3 TCP Phase-1 [FAILED]
    exit 1
fi
echo "CLUSTER-3 SCTP Start"
code=0
code=$(sctp_validate $master)
if [[ $code == 0 ]]
then
    echo CLUSTER-3 SCTP Phase-1 [OK]
    docker stop $master
else
    echo CLUSTER-3 SCTP Phase-1 [FAILED]
    exit 1
fi

sleep 2

rnh=$($hexec r1 ip route list match 20.20.20.1 | grep "proto zebra" | cut -d ' ' -f 3)

if [[ ${llbIp[$backup]} == "$rnh" ]]
then
    echo CLUSTER-3 HA [SUCCESS]
else
    echo CLUSTER-3 HA [FAILED]
    exit 1
fi
code=0
code=$(tcp_validate $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-3 TCP [OK]
else
    echo CLUSTER-3 TCP [FAILED]
    exit 1
fi

code=$(sctp_validate $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-3 SCTP [OK]
else
    echo CLUSTER-3 SCTP [FAILED]
fi

exit $code
