#!/bin/bash
source ../common.sh
echo SCENARIO-sctplb

servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )

$hexec l3ep1 socat -v -T0.5 sctp-l:8080,reuseaddr,fork system:"echo 'server1'; cat" >/dev/null 2>&1 &
$hexec l3ep2 socat -v -T0.5 sctp-l:8080,reuseaddr,fork system:"echo 'server2'; cat" >/dev/null 2>&1 &
$hexec l3ep3 socat -v -T0.5 sctp-l:8080,reuseaddr,fork system:"echo 'server3'; cat" >/dev/null 2>&1 &

sleep 5
code=0
j=0
waitCount=0
while [ $j -le 2 ]
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_socat_client 10.10.10.1 0 ${ep[j]} 8080)
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
            echo SCENARIO-sctplb [FAILED]
            sudo pkill sctp_server >/dev/null 2>&1
            exit 1
        fi

    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..2}
do
    res=$($hexec l3h1 timeout 10 ../common/sctp_socat_client 10.10.10.1 0 20.20.20.1 2020)
    echo -e $res
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
    sleep 1
done
done
sudo pkill socat >/dev/null 2>&1
sudo pkill sctp_server >/dev/null 2>&1
if [[ $code == 0 ]]
then
    echo SCENARIO-sctplb [OK]
else
    echo SCENARIO-sctplb [FAILED]
fi
exit $code

