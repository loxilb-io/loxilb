#!/bin/bash
source ../common.sh

sudo pkill socat

$hexec l3ep1 socat udp-listen:8080,fork exec:'echo server1' &
$hexec l3ep2 socat udp-listen:8080,fork exec:'echo server2' &
$hexec l3ep3 socat udp-listen:8080,fork exec:'echo server3' &

$dexec llb1 loxicmd create lb 20.20.20.1 --udp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1
sleep 4
code=0
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
for i in {1..5}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 1 ./client 20.20.20.1 2020)
    #res=$($hexec l3h1 timeout 1 ./client ${ep[j]} 8080)
    echo -e $res
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
done
done
if [[ $code == 0 ]]
then
    echo [OK]
else
    echo [FAILED]
fi
sudo pkill socat
exit $code

