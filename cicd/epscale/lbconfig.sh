#!/bin/bash
for i in {1..3}
do
for j in {1..255}
do
  loxicmd create lb 20.$i.$j.1 --tcp=2020:8080 --endpoints=35.$i.$j.1:1 --monitor 2>&1 > /dev/null
done
done

