#!/bin/bash
source ../common.sh
echo SCENARIO-SCTP-FULLNAT
servArr=( "server1" "server2" )
ep=( "10.0.3.10" "10.0.3.11" )

$hexec ep1 ../common/sctp_server ${ep[0]} 38412 server1 >/dev/null 2>&1 &
$hexec ep2 ../common/sctp_server ${ep[1]} 38412 server2 >/dev/null 2>&1 &

sleep 5

code=0
j=0
waitCount=0
while [ $j -le 1 ]
do
    res=$($hexec c1 timeout 10 ../common/sctp_client 10.0.3.71 0 ${ep[j]} 38412)
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
            echo SCENARIO-SCTP-FULLNAT [FAILED]
            sudo pkill -9 -x  sctp_server >/dev/null 2>&1
            exit 1
        fi

    fi
    sleep 1
done

for i in {1..4}
do
for j in {0..1}
do
    res=$($hexec c1 timeout 10 ../common/sctp_client 10.0.3.71 0 20.20.20.1 38412)
    echo -e $res
    if [[ $res != "${servArr[j]}" ]]
    then
        code=1
    fi
    sleep 1
done
done
if [[ $code == 0 ]]
then
    echo SCENARIO-SCTP-FULLNAT [OK]
else
    echo SCENARIO-SCTP-FULLNAT [FAILED]
fi
sudo pkill -9 -x  sctp_server >/dev/null 2>&1
exit $code

