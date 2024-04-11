#!/bin/bash
source ../common.sh
echo SCENARIO-sctplbdsr
servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )

$hexec l3ep1 socat -v -T0.5 sctp-l:2020,reuseaddr,fork system:"echo 'server1'; cat" >/dev/null 2>&1 &
$hexec l3ep2 socat -v -T0.5 sctp-l:2020,reuseaddr,fork system:"echo 'server2'; cat" >/dev/null 2>&1 &
$hexec l3ep3 socat -v -T0.5 sctp-l:2020,reuseaddr,fork system:"echo 'server3'; cat" >/dev/null 2>&1 &

#$hexec l3ep1 ../common/sctp_server 20.20.20.1,31.31.31.1 2020 server1 >/dev/null 2>&1 &
#$hexec l3ep2 ../common/sctp_server 20.20.20.1,32.32.32.1 2020 server2 >/dev/null 2>&1 &
#$hexec l3ep3 ../common/sctp_server 20.20.20.1,33.33.33.1 2020 server3 >/dev/null 2>&1 &

sleep 5
code=0
j=0
waitCount=0
while [ $j -le 2 ]
do
    #res=$($hexec l3h1 socat -T10 - SCTP:${ep[j]}:2020)
    res=$($hexec l3h1 timeout 10 ../common/sctp_socat_client 10.10.10.1 2010 ${ep[j]} 2020)
    echo $res
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
            echo SCENARIO-sctplbdsr [FAILED]
            sudo pkill -9 sctp_server > /dev/null 2>&1
            exit 1
        fi
    fi
    sleep 1
done

#sudo killall -9 sctp_server >/dev/null 2>&1

#$hexec l3ep1 ../common/sctp_server 20.20.20.1 2020 server1 >/dev/null 2>&1 &
#$hexec l3ep2 ../common/sctp_server 20.20.20.1 2020 server2 >/dev/null 2>&1 &
#$hexec l3ep3 ../common/sctp_server 20.20.20.1 2020 server3 >/dev/null 2>&1 &

sleep 5

nid=0
for j in {0..2}
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_socat_client 10.10.10.1 2010 20.20.20.1 2020)
    echo $res
    if [[ $exp == "" ]]
    then
      exp=$res
    fi
    if [[ $exp != $res ]]
    then
      code=1
    fi
    sleep 1
done
if [[ $code == 0 ]]
then
    echo SCENARIO-sctplbdsr [OK]
else
    echo SCENARIO-sctplbdsr [FAILED]
fi

sudo pkill -9 socat >> /dev/null 2>&1
sudo pkill -9 sctp_server >/dev/null 2>&1
exit $code
