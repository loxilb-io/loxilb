#!/bin/bash
source ../common.sh
echo IPSEC-e2e
$hexec rh1 node ../common/tcp_server.js server1 &
$hexec rh2 node ../common/tcp_server.js server2 &

sleep 2
lgw1_rx1=`$hexec lgw1 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
lgw1_tx1=`$hexec lgw1 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`
llb1_rx1=`$hexec llb1 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
llb1_tx1=`$hexec llb1 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`
llb1_rx2=`$hexec llb1 ifconfig vti200 | grep "RX packets" | cut -d " " -f 11`
llb1_tx2=`$hexec llb1 ifconfig vti200 | grep "TX packets" | cut -d " " -f 11`
llb1_rx3=`$hexec llb1 ifconfig vti201 | grep "RX packets" | cut -d " " -f 11`
llb1_tx3=`$hexec llb1 ifconfig vti201 | grep "TX packets" | cut -d " " -f 11`

rgw1_rx1=`$hexec rgw1 ifconfig vti200 | grep "RX packets" | cut -d " " -f 11`
rgw1_tx1=`$hexec rgw1 ifconfig vti200 | grep "TX packets" | cut -d " " -f 11`
rgw2_rx1=`$hexec rgw2 ifconfig vti201 | grep "RX packets" | cut -d " " -f 11`
rgw2_tx1=`$hexec rgw2 ifconfig vti201 | grep "TX packets" | cut -d " " -f 11`

code=0
servArr=( "server1" "server2" )
vip=( "192.168.10.200" )

for j in {0..3}
do
for i in {0..1}
do
    res=`$hexec lh1 curl --max-time 10 -s http://${vip[0]}:2020`
    echo -e $res
    if [[ "x$res" != "x${servArr[$i]}" ]]
    then
        echo -e "Expected ${servArr[$i]}, Received : $res"
        if [[ "$res" != *"server"* ]];
        then
            echo "lgw1 ct"
            $dexec lgw1 loxicmd get ct
            echo "llb1 ct"
            $dexec llb1 loxicmd get ct
            echo "rgw1 ct"
            $dexec rgw1 loxicmd get ct
            echo "rgw2 ct"
            $dexec rgw2 loxicmd get ct
            echo "llb1 ip neigh"
            $dexec llb1 ip neigh
        fi
        code=1
    fi
    sleep 1
done
done
if [[ $code == 0 ]]
then
    echo IPSEC-3 [OK]
else
    echo IPSEC-3 [FAILED]
fi
sudo pkill node
exit $code

