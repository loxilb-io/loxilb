#!/bin/bash
echo $1 $2 is in $3 state > /root/keepalive.state
curl -X 'POST' 'http://0.0.0.0:11111/netlox/v1/config/cistate' -H 'accept: application/json' -H 'Content-Type: application/json' -d '{ "instance": "'$2'", "state" : "'$3'" }'
