#!/bin/bash

source ../common.sh

if [[ $# != 1 ]]; then
    echo "Enter scenario name to be tested"
    exit 1;
fi

if [[ ! -d "../$1" ]]; then
    echo "Test scenario $1 doesn't exist";
    exit 1;
fi

name=$1

echo "Executing Persistent Config Test for $name"

cd ../$name
. ./config.sh
./validation.sh
if [[ $? == 0 ]]; then
    echo "$name [OK]"
else
    echo "$name [FAILED]"
    #./rmconfig.sh
    exit 1
fi

llbs=( "${loxilbs[*]}" )
echo ${llbs[*]}
for llb in ${llbs[@]}
do 
    echo "Saving configs for $llb"
    $dexec $llb loxicmd save -a
    sudo rm -rf ${llb}_config
    docker cp $llb:/etc/loxilb ${llb}_config
done
. ./rmconfig.sh
pick_config="yes"
sleep 1
. ./config.sh
./validation.sh
if [[ $? == 0 ]]; then
    echo "Persistent Config Test for $name [OK]"
else
    echo "Persistent Config Test for $name [FAILED]"
fi
for llb in ${llbs[@]}
do 
    rm -rf ${llb}_config
done
. ./rmconfig.sh
