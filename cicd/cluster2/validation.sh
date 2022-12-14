#!/bin/bash
source ../common.sh
echo CLUSTER-2

function tcp_validate() {
  $hexec ep1 node ../common/tcp_server.js server1 &
  $hexec ep2 node ../common/tcp_server.js server2 &
  $hexec ep3 node ../common/tcp_server.js server3 &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )
  declare -A llbIp

  llbIp["llb1"]="11.11.11.1"
  llbIp["llb2"]="11.11.11.2"
  echo "Master-$1(${llbIp[$1]})" >&2
  $hexec r1 ip route list match 20.20.20.1 | grep -w ${llbIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route list match 20.20.20.1 >&2
    echo "BGP Service Route [NOK]" >&2
    sudo pkill node
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
  sudo pkill node
  echo $code
}

function sctp_validate() {
  $hexec ep1 ../common/sctp_server server1 &
  $hexec ep2 ../common/sctp_server server2 &
  $hexec ep3 ../common/sctp_server server3 &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )
  declare -A llbIp

  llbIp["llb1"]="11.11.11.1"
  llbIp["llb2"]="11.11.11.2"
  echo "Master-$1(${llbIp[$1]})" >&2
  $hexec r1 ip route list match 20.20.20.1 | grep -w ${llbIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route list match 20.20.20.1 >&2
    echo "BGP Service Route [NOK]" >&2
    sudo pkill sctp_server
    return 1
  fi 

  for i in {1..5}
  do
  for j in {0..2}
  do
    res=$($hexec user timeout 10 ../common/sctp_client 20.20.20.1 2020)
    echo -e $res >&2
    if [[ $res != "${servArr[j]}" ]]
    then
        echo "Expected : "${servArr[j]}", Received : $res" 2>&1
        code=1
    fi
    sleep 1
  done
  done
  sudo pkill sctp_server
  echo $code
}

count=0
while : ; do
  status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
  status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
  if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
  then
    master="llb1"
    backup="llb2"
    break
  elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]];
  then
    master="llb2"
    backup="llb1"
    break
  else
    echo CLUSTER-2 HA state llb1-$status1 llb2-$status2 [FAILED]
    sleep 10
    count=$(( $count + 1 ))
    if [[ $count -ge 20 ]]; then
      echo "KeepAlive llb1-$status1, llb2-$status2 [NOK]"
      exit 1;
    fi
    if [[ $status1 != "MASTER" || $status1 != "BACKUP" ]]; then
      docker restart ka_llb1
    fi
    if [[ $status2 != "MASTER" || $status2 != "BACKUP" ]]; then
      docker restart ka_llb2
    fi
  fi
done

echo CLUSTER-2 HA state llb1-$status1 llb2-$status2
echo "Master:$master Backup:$backup"


count=0
while : ; do
  $dexec $master gobgp neigh | grep "Estab" 2>&1 >> /dev/null
  if [[ $? -eq 0 ]]; then
    echo "$master BGP connection [OK]"
    break;
  fi
  sleep 0.1
  count=$(( $count + 1 ))
  if [[ $count -ge 1000 ]]; then
    echo "$master BGP connection [NOK]"
    exit 1;
  fi
done

count=0
while : ; do
  $dexec $backup gobgp neigh | grep "Estab" >> /dev/null
  if [[ $? -eq 0 ]]; then
    echo "$backup BGP connection [OK]"
    break;
  fi
  sleep 0.1
  count=$(( $count + 1 ))
  if [[ $count -ge 1000 ]]; then
    echo "$backup BGP connection [NOK]"
    exit 1;
  fi
done

echo "CLUSTER-2 TCP Start"
code=0
code=$(tcp_validate $master)
if [[ $code == 0 ]]
then
    echo CLUSTER-2 TCP Phase-1 [OK]
    docker restart ka_$master
else
    echo CLUSTER-2 TCP Phase-1 [FAILED]
    exit 1
fi

sleep 2

status1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
status2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
echo CLUSTER-2 HA state $master-$status1 $backup-$status2

if [[ $status2 == "MASTER" && $status1 == "BACKUP" ]];
then
    echo CLUSTER-2 HA [SUCCESS]
else
    echo CLUSTER-2 HA [FAILED]
    exit 1
fi
code=0
code=$(tcp_validate $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-2 TCP [OK]
else
    echo CLUSTER-2 TCP [FAILED]
    exit 1
fi

echo "CLUSTER-2 SCTP Start"
status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
echo CLUSTER-2 HA state llb1-$status1 llb2-$status2

if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
then
    master="llb1"
    backup="llb2"
elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; 
then
    master="llb2"
    backup="llb1"
else
    echo CLUSTER-2 HA state llb1-$status1 llb2-$status2 [FAILED]
fi
echo "Master:$master Backup:$backup"

code=0
code=$(sctp_validate $master)
if [[ $code == 0 ]]
then
    echo CLUSTER-2 SCTP Phase-1 [OK]
    docker restart ka_$master
else
    echo CLUSTER-2 SCTP Phase-1 [FAILED]
fi

sleep 2

status1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
status2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
echo CLUSTER-2 HA state $master-$status1 $backup-$status2

if [[ $status2 == "MASTER" && $status1 == "BACKUP" ]];
then
    echo CLUSTER-2 SCTP HA [SUCCESS]
else
    echo CLUSTER-2 SCTP HA [FAILED]
    exit 1
fi

code=$(sctp_validate $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-2 SCTP [OK]
else
    echo CLUSTER-2 SCTP [FAILED]
fi

exit $code
