#!/bin/bash
source ../common.sh
echo CLUSTER-3
declare -A llbExIp
llbExIp["llb1"]="11.11.11.1"
llbExIp["llb2"]="11.11.11.2"

declare -A llbInIp
llbInIp["llb1"]="10.10.10.1"
llbInIp["llb2"]="10.10.10.2"

ep=( "10.10.10.3" "10.10.10.4" "10.10.10.5" )
function tcp_validate() {
  $hexec ep1 node ../common/tcp_server.js server1 &
  $hexec ep2 node ../common/tcp_server.js server2 &
  $hexec ep3 node ../common/tcp_server.js server3 &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )

  echo "Master-$1(${llbExIp[$1]})" >&2
  $hexec r1 ip route list match 20.20.20.1 | grep -w ${llbExIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route list match 20.20.20.1 >&2
    echo "BGP Service Route [NOK]" >&2
    sudo pkill node
    return 1
  fi 

  $hexec r1 ip route replace 10.10.10.0/24 via ${llbExIp[$1]}
  $hexec r2 ip route replace 1.1.1.0/24 via ${llbInIp[$1]}
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
    res=$($hexec user curl --max-time 10 -s 20.20.20.1:2020)
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
        echo "Expected server$nid got server$((ids))" >&2
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
  echo $code
}

function sctp_validate() {
  $hexec ep1 ../common/sctp_server ${ep[0]} 8080 server1 >/dev/null 2>&1 &
  $hexec ep2 ../common/sctp_server ${ep[1]} 8080 server2 >/dev/null 2>&1 &
  $hexec ep3 ../common/sctp_server ${ep[2]} 8080 server3 >/dev/null 2>&1 &

  sleep 20

  local code=0
  servArr=( "server1" "server2" "server3" )

  echo "Master-$1(${llbExIp[$1]})" >&2
  $hexec r1 ip route list match 20.20.20.1 | grep -w ${llbExIp[$1]} 2>&1 >/dev/null
  if [[ $? -eq 0 ]]; then
    echo "BGP Service Route [OK]" >&2
  else
    $hexec r2 ip route list match 20.20.20.1 >&2
    echo "BGP Service Route [NOK]" >&2
    sudo pkill sctp_server >/dev/null 2>&1
    return 1
  fi 
  
  $hexec r1 ip route replace 10.10.10.0/24 via ${llbExIp[$1]}
  $hexec r2 ip route replace 1.1.1.0/24 via ${llbInIp[$1]}
  j=0
  waitCount=0
  while [ $j -le 2 ]
  do
    res=$($hexec user timeout 10 ../common/sctp_client 1.1.1.1 0 ${ep[j]} 8080)
    if [[ $res == "${servArr[j]}" ]]
    then
        echo "$res UP" >&2
        j=$(( $j + 1 ))
    else
        echo "Waiting for ${servArr[j]}(${ep[j]})" >&2
        waitCount=$(( $waitCount + 1 ))
        if [[ $waitCount == 10 ]];
        then
            echo "All SCTP Servers are not UP" >&2
            echo CLUSTER-2 [FAILED] >&2
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
    res=$($hexec user timeout 10 ../common/sctp_client 1.1.1.1 0 20.20.20.1 2020)
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
        echo "Expected server$nid got server$((ids))" >&2
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
  sudo pkill sctp_server >/dev/null 2>&1
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


count=0
while : ; do
  rnh=$($hexec r1 ip route list match 20.20.20.1 | grep "proto zebra" | cut -d ' ' -f 3)

  if [[ $rnh == "11.11.11.1" ]];
  then
    master="llb1"
    backup="llb2"
    break
  elif [[ $rnh == "11.11.11.2" ]]; 
  then
    master="llb2"
    backup="llb1"
    break
  else
    sleep 0.2
    count=$(( $count + 1 ))
    if [[ $count -ge 2000 ]]; then
      $hexec r1 ip route list match 20.20.20.1
      echo CLUSTER-3 Service Route not advertised to External Router [FAILED]
      exit 1;
    fi
  fi
done

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

sleep 10

rnh=$($hexec r1 ip route list match 20.20.20.1 | grep "proto zebra" | cut -d ' ' -f 3)

if [[ ${llbExIp[$backup]} == "$rnh" ]]
then
    echo CLUSTER-3 HA [SUCCESS]
else
    $hexec r1 ip route list match 20.20.20.1
    echo CLUSTER-3 HA [FAILED]
    exit 1
fi
echo "CLUSTER-3 TCP Start"
code=0
code=$(tcp_validate $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-3 TCP [OK]
else
    echo CLUSTER-3 TCP [FAILED]
    exit 1
fi

echo "CLUSTER-3 SCTP Start"
code=$(sctp_validate $backup)
if [[ $code == 0 ]]
then
    echo CLUSTER-3 SCTP [OK]
else
    echo CLUSTER-3 SCTP [FAILED]
fi

exit $code
