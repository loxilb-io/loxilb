#!/bin/bash
source ../common.sh
echo IPSEC-3
$hexec rh1 node ../common/tcp_server.js server1 &

sleep 10
llb1_rx1=`$hexec llb1 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
llb1_tx1=`$hexec llb1 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`
llb2_rx1=`$hexec llb2 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
llb2_tx1=`$hexec llb2 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`

code=0
servArr=( "server1" )
vip=( "192.168.10.200" )

for i in {1..2}
do
    res=`$hexec lh1 curl --max-time 10 -s http://${vip[0]}:2020`
    echo -e $res
    if [[ "x$res" != "x${servArr[0]}" ]]
    then
        echo -e "Expected ${servArr[0]}, Received : $res"
        if [[ "$res" != *"server"* ]];
        then
            echo "llb1 ct"
            $dexec llb1 loxicmd get ct
            echo "llb2 ct"
            $dexec llb2 loxicmd get ct
            echo "llb2 ip neigh"
            $dexec llb2 ip neigh
        fi
        code=1
    fi
    sleep 1
done
if [[ $code == 0 ]]
then
    echo IPSEC-3 [OK]
else
    echo IPSEC-3 [FAILED]
fi
sudo pkill node
exit $code

