#!/bin/bash
source ../common.sh
$dexec llb1 loxicmd create lb 20.20.20.1 --sctp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1

