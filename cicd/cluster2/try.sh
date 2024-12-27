#!/bin/bash
declare -A ip
ip["llb1"]="10.10.10.1"
ip["llb2"]="10.10.10.1"
echo ${ip["llb1"]}
