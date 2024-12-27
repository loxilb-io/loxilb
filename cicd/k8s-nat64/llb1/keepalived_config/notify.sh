#!/bin/bash
logfile=/var/log/notify.log
exec >> $logfile 2>&1
set -x
date=`date`
declare -A vip
vip["default"]="192.168.58.150"
echo $1 $2 is in $3 state vip ${vip[$2]}> /etc/shared/keepalive.state
curl -X 'POST' 'http://0.0.0.0:11111/netlox/v1/config/cistate' -H 'accept: application/json' -H 'Content-Type: application/json' -d '{ "instance": "'$2'", "state" : "'$3'", "vip" : "'${vip[$2]}'" }'
