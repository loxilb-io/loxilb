#!/bin/bash
source ../common.sh
echo IPSEC-1
$hexec rh1 node ../common/tcp_server.js server1 &
$hexec rh2 node ../common/tcp_server.js server2 &

sleep 10
llb1_rx1=`$hexec llb1 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
llb1_tx1=`$hexec llb1 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`
llb2_rx1=`$hexec llb2 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
llb2_tx1=`$hexec llb2 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`

code=0
servArr=( "server1" "server2" )
ep=( "25.25.25.1" "26.26.26.1" )
i=1
while [ $i -le 2 ]
do
j=0
while [ $j -le 1 ]
do
    $hexec lh$i ping ${ep[j]} -f -c 500 -W 1 2>&1 > /dev/null
    if [[ $? -eq 0 ]]
    then
      printf "Ping %-16s \t->\t %-16s \t\t: [OK]\n" lh$i ${servArr[j]} $size ;
    else
      printf "Ping %-16s \t->\t %-16s \t\t: [OK]\n" lh$i ${servArr[j]} $size ;
      code=1
    fi
    j=$(( $j + 1 ))
done
    i=$(( $i + 1 ))
done

if [[ $code == 0 ]]
then
   llb1_rx2=`$hexec llb1 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
   llb1_tx2=`$hexec llb1 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`
   llb2_rx2=`$hexec llb2 ifconfig vti100 | grep "RX packets" | cut -d " " -f 11`
   llb2_tx2=`$hexec llb2 ifconfig vti100 | grep "TX packets" | cut -d " " -f 11`
   if [[ $(expr $llb1_rx2 - $llb1_rx1) != 2000 ||
         $(expr $llb2_rx2 - $llb2_rx1) != 2000 ||
         $(expr $llb1_tx2 - $llb1_tx1) != 2000 ||
         $(expr $llb2_tx2 - $llb2_tx1) != 2000 ]]; then
     echo "IPSec Tunnel Traffic [NOK]"
     echo "IPSEC-1 [FAILED]"
     exit 1;
   else
     echo "IPSec Tunnel Traffic [OK]"
   fi
else
    echo "IPSEC-1 [FAILED]"
    sudo pkill node
    exit $code
fi

waitCount=0
j=0
while [ $j -le 1 ]
do
    #res=$($hexec h1 curl ${ep[j]}:8080)
    res=`$hexec lh1 curl --max-time 10 -s ${ep[j]}:8080`
    #echo $res
    if [[ $res == "${servArr[j]}" ]]
    then
        echo "$res UP"
        j=$(( $j + 1 ))
    else
        echo "Waiting for ${servArr[j]}(${ep[j]})"
        waitCount=$(( $waitCount + 1 ))
        if [[ $waitCount == 10 ]];
        then
            echo "All Servers are not UP"
            echo IPSEC-1 [FAILED]
            sudo pkill node
            exit 1
        fi
    fi
    sleep 1
done

for k in {1..2}
do
for i in {1..2}
do
for j in {0..1}
do
    #$hexec h$k ping ${ep[j]} -f -c 5 -W 1;
    res=`$hexec lh$k curl --max-time 10 -s 20.20.20.1:2020`
    #echo -e $res
    if [[ $res != "${servArr[j]}" ]]
    then
        echo -e "Expected ${servArr[j]}, Received : $res"
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
done
done
if [[ $code == 0 ]]
then
    echo IPSEC-1 [OK]
else
    echo IPSEC-1 [FAILED]
fi
sudo pkill node
exit $code

