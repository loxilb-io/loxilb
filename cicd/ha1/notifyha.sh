#!/bin/bash
echo $1 $2 is in $3 state > /root/keepalive.state
curl -X 'POST' 'http://0.0.0.0:11111/netlox/v1/config/hastate' -H 'accept: application/json' -H 'Content-Type: application/json' -d '{ "state" : "'$3'" }'
