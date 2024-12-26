#!/bin/bash
source ../common.sh
echo SCENARIO-egrlb

sleep 10 
code=0

check_egr_conn() {
    IP="$1"
    if $hexec h1 curl -s --head --connect-timeout 5 http://$IP | grep -i "nginx" > /dev/null; then
        echo "Connectivity to http://$IP is OK."
    else
        echo "Failed to connect to https://$IP"
        code=1
    fi
}

check_ping() {
    IP="$1"
    if $hexec h1 ping -c 5 -W 10 "$IP" &> /dev/null; then
        echo "Ping to $IP is OK."
    else
        echo "Ping to $IP failed."
        code=1
    fi
}

echo "Checking egress before HA"

PING_IP_ADDRESS="8.8.8.8"
CURL_IP_ADDRESS="208.67.222.222"
#check_ping $PING_IP_ADDRESS
check_egr_conn $CURL_IP_ADDRESS

$hexec llb2 curl -X 'POST' \
  'http:/127.0.0.1:11111/netlox/v1/config/cistate' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "instance": "default",
  "state": "BACKUP",
  "vip": "0.0.0.0"
}'

$hexec llb1 curl -X 'POST' \
  'http://127.0.0.1:11111/netlox/v1/config/cistate' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "instance": "default",
  "state": "MASTER",
  "vip": "0.0.0.0"
}'

sleep 10
echo "Checking egress after HA"

#check_ping $IP_ADDRESS
check_egr_conn $CURL_IP_ADDRESS

if [[ $code != 0 ]];then
  echo "SCENARIO-egrlb FAILED"
else
  echo "SCENARIO-egrlb OK"
fi
