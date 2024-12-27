#!/bin/bash

for ((i=1,port=12865;i<=150;i++,port++)); do echo "frontend nperf$port
  bind *:$port
  mode tcp
  option tcplog
  use_backend nperf-endpoints$port

backend nperf-endpoints$port
  mode tcp
  balance roundrobin
  server server1 31.31.31.1:$port
" >> /etc/haproxy/haproxy.cfg; done
