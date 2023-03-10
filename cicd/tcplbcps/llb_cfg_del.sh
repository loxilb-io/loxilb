#!/bin/bash

for ((i=1,port=12865;i<=150;i++,port++))
do
  loxicmd delete lb 20.20.20.1 --tcp $port >> /dev/null
done
