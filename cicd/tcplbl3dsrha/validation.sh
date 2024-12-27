#!/bin/bash
source ../common.sh
echo SCENARIO-tcplbl3dsrha
$hexec ep1 socat -v -T0.05 tcp-l:8080,reuseaddr,fork system:"echo 'server1'; cat" >/dev/null 2>&1 &
$hexec ep2 socat -v -T0.05 tcp-l:8080,reuseaddr,fork system:"echo 'server2'; cat" >/dev/null 2>&1 &
$hexec ep3 socat -v -T0.05 tcp-l:8080,reuseaddr,fork system:"echo 'server3'; cat" >/dev/null 2>&1 &

function wait_vip_ready {
  i=1
  nr=0
  for ((;;)) do
    res=$($hexec r1 ip route |grep 20.20.20.1)
    if [[ x$res != x"" ]]; then
      echo "VIP advertised"
      break
    fi

    i=$(( $i + 1 ))
    if [[ $i -ge 40 ]]; then
        echo "VIP not found in r1. Giving up"
        exit 1
    fi
    echo "VIP not found in r1.Waiting..."
    sleep 10
  done
}

wait_vip_ready
sleep 5
code=0
exp=""
for i in {1..4}
do
for j in {0..2}
do
    #res=$($hexec user curl --local-port 55001 --max-time 10 -s 20.20.20.1:2020)
    res=$($hexec user socat -T2 - TCP:20.20.20.1:8080,sp=15402,reuseaddr)
    echo $res
    if [[ $exp == "" ]]
    then
      exp=$res
    fi
    if [[ $exp != $res ]]
    then
      code=1
    fi
done
done
if [[ $code == 0 ]]
then
    echo SCENARIO-tcplbl3dsrha [OK]
else
    echo SCENARIO-tcplbl3dsrha [FAILED]
fi
sudo pkill -9 socat 2>&1 > /dev/null
exit $code

