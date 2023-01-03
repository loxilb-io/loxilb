#!/bin/bash
source ../common.sh
echo CLUSTER-1


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
    sleep 5
  fi
done

echo CLUSTER-1 HA state llb1-$status1 llb2-$status2
echo "Master:$master Backup:$backup"
    COLUMNS="`tput cols`"
    LINES="`tput lines`"

    pid=$( docker exec -e $COLUMNS -e $LINES $master ps -aef | grep "/root/loxilb" | cut -d ' ' -f 11 )
    
    ps=$( docker exec $master ps -aef )
    echo "pid : " $pid 
    echo "ps : " $ps 
    #$dexec $master kill -9 $pid
    opts=$( docker exec -e COLUMNS="`tput cols`" -e LINES="`tput lines`" $master ps -aef | grep "/root/loxilb" | cut -d ' ' -f 28- )
    echo $opts
    #docker exec -dt $master /root/loxilb-io/loxilb/loxilb $opts

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

exit $code
