#!/bin/bash
source ../common.sh
echo CLUSTER-1

function myfunc() {
  $hexec ep1 node ./server1.js &
  $hexec ep2 node ./server2.js &
  $hexec ep3 node ./server3.js &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )
  declare -A llbIp

  llbIp["llb1"]="10.10.10.1"
  llbIp["llb2"]="10.10.10.2"

  $hexec r2 ip route list match 20.20.20.1 | grep ${llbIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route match 20.20.20.1
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
        code=1
    fi
    sleep 1
  done
  done
  echo $code
}

while : ; do
  status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
  status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
  count=0
  if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
  then
    master="llb1"
    backup="llb2"
  elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; 
  then
    master="llb2"
    backup="llb1"
  else
    echo CLUSTER-1 HA state llb1-$status1 llb2-$status2 [FAILED]
    sleep 0.2
    count=$(( $count + 1 ))
    
    if [[ $count -ge 2000 ]]; then
      echo "KeepAlive llb1-$status1, llb2-$status2 [NOK]"
      exit 1;
    fi
  fi
done

echo CLUSTER-1 HA state llb1-$status1 llb2-$status2
echo "Master:$master Backup:$backup"

count=0
while : ; do
  $dexec $master gobgp neigh | grep "Estab" 2>&1 >> /dev/null
  if [[ $? -eq 0 ]]; then
    echo "$master BGP connection [OK]"
    break;
  fi
  sleep 0.2
  count=$(( $count + 1 ))
  if [[ $count -ge 2000 ]]; then
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
  sleep 0.2
  count=$(( $count + 1 ))
  if [[ $count -ge 2000 ]]; then
    echo "$backup BGP connection [NOK]"
    exit 1;
  fi
done

code=$(myfunc $master)
if [[ $code == 0 ]]
then
    echo CLUSTER-1 Phase-1 [OK]
    docker restart ka_$master
else
    echo CLUSTER-1 Phase-1 [FAILED]
fi

sleep 2

status1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
status2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
echo CLUSTER-1 HA state $master-$status1 $backup-$status2

if [[ $status2 == "MASTER" && $status1 == "BACKUP" ]];
then
    echo CLUSTER-1 HA [SUCCESS]
else
    echo CLUSTER-1 HA [FAILED]
    exit 1
fi

code=$(myfunc $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-1 [OK]
else
    echo CLUSTER-1 [FAILED]
fi

exit $code
