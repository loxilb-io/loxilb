#!/bin/bash
COLUMNS="`tput cols`"
LINES="`tput lines`"
master=llb1
pid=`docker exec -e $COLUMNS -e $LINES llb1 ps -aef | grep "/root/loxilb" | cut -d ' ' -f 11`
echo pid: $pid

