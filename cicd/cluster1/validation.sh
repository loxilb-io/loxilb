#!/bin/bash
source ../common.sh
echo CLUSTER-1

function myfunc() {
  $hexec ep1 node ../common/tcp_server.js server1 &
  $hexec ep2 node ../common/tcp_server.js server2 &
  $hexec ep3 node ../common/tcp_server.js server3 &
  
  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )
  ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
  declare -A llbIp

  llbIp["llb1"]="10.10.10.1"
  llbIp["llb2"]="10.10.10.2"

  $hexec r2 ip route list match 20.20.20.1 | grep ${llbIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route match 20.20.20.1
    echo "BGP Service Route [NOK]" >&2
    sudo pkill node
    return 1
  fi
  $hexec r2 ip route replace 1.1.1.0/24 via ${llbIp[$1]}
  j=0
  waitCount=0
  while [ $j -le 2 ]
  do
    res=$($hexec user curl --max-time 10 -s ${ep[j]}:8080)
    if [[ $res == "${servArr[j]}" ]]
    then
        echo "$res UP" >&2
        j=$(( $j + 1 ))
    else
        echo "Waiting for ${servArr[j]}(${ep[j]})" >&2
        waitCount=$(( $waitCount + 1 ))
        if [[ $waitCount == 11 ]];
        then
            echo "All TCP Servers are not UP" >&2
            echo CLUSTER-2 [FAILED] >&2
            sudo pkill node
            exit 1
        fi
    fi
    sleep 1
  done
  nid=0
  for i in {1..4}
  do
  for j in {0..2}
  do
    res=$($hexec user timeout 1 curl --max-time 10 -s 20.20.20.1:2020)
    echo -e $res >&2
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
        code=1
      fi
      nid=$((($ids + 1)%4))
      if [[ $nid == 0 ]];then
        nid=1
      fi
    else
      code=1
    fi

    sleep 1
  done
  done
  sudo pkill node
  echo "$code"
}

while : ; do
  status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
  status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
  count=0
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
    echo CLUSTER-1 HA state llb1-$status1 llb2-$status2 [FAILED]
    count=$(( $count + 1 ))
    if [[ $count -ge 20 ]]; then
      echo "KeepAlive llb1-$status1, llb2-$status2 [NOK]"
      exit 1;
    fi
    if [[ $status1 != "MASTER" && $status1 != "BACKUP" ]]; then
      docker restart ka_llb1
    fi
    if [[ $status1 != "MASTER" && $status1 != "BACKUP" ]]; then
      docker restart ka_llb2
    fi
    sleep 10
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

#code=$(myfunc $master)
code=`myfunc $master`
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
