#!/bin/bash
source ../common.sh
echo SCENARIO-SCTP-ONEARM
servArr=( "server1" "server2" )
ep=( "10.75.188.218" "10.75.188.220" )
$hexec ep1 socat -v -T0.5 sctp-l:38412,reuseaddr,fork system:"echo 'server1'; cat" >/dev/null 2>&1 &
$hexec ep2 socat -v -T0.5 sctp-l:38412,reuseaddr,fork system:"echo 'server2'; cat" >/dev/null 2>&1 &

sleep 60
$dexec llb1 loxicmd get ep
sleep 10

code=0
# Below code checks the client-server connectivity and resolves ARP. 
# For this test case, We don't want ARP to be resolved, so keeping the code with initial value j=2
# If someone wants to run the test with ARP resolved then simply do j=0 and execute the script.
j=2
waitCount=0
while [ $j -le 1 ]
do
    res=$($hexec c1 timeout 10 ../common/sctp_socat_client 10.75.191.224 0 ${ep[j]} 38412)
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
            echo SCENARIO-SCTP-ONEARM [FAILED]
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
    res=$($hexec c1 timeout 10 ../common/sctp_socat_client 10.75.191.224 0 123.123.123.1 38412)
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
    echo SCENARIO-SCTP-ONEARM [OK]
else
    echo SCENARIO-SCTP-ONEARM [FAILED]
fi
sudo pkill -9 -x  socat >/dev/null 2>&1
sudo pkill -9 -x  sctp_server >/dev/null 2>&1
exit $code

