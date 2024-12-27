#!/bin/bash

for ((i=1,port=12865;i<=150;i++,port++))
do
  loxicmd create lb 20.20.20.1 --tcp=$port:$port  --endpoints=31.31.31.1:1 >> /dev/null
done
