#!/bin/bash
source ../common.sh

rm -fr port*.log
echo SCENARIO-udplb-iperf3
for ((i=1;i<3;i++))
do
for ((port=8001;port<=8100;port++))
do
$hexec l3ep$i iperf3 -s -p $port --logfile port$port-ep$i.log 2>&1 >> /dev/null &
done
done

echo "Waiting for servers to start..."
sleep 30 

NUM_HOSTS=30
rm -fr iperf-*.log
for ((i=1;i<=$NUM_HOSTS;i++))
do
$hexec l3h$i iperf3 -c 150.150.150.1 -p 2020 -u -t20 --logfile iperf-$i.log --forceflush &
done

echo "Waiting for tests to finish..."
sleep 60 
code=0
for file in iperf*.log; do
  if grep -q "connected" "$file"; then
    echo "Pass:'connected' found in $file."
  else
    echo "Fail: 'connected' not found in $file."
    code=1
  fi
done
if [[ $code != 0 ]]; then
    echo "SCENARIO-udplb-iperf3 [FAILED]"
else
    echo "SCENARIO-udplb-iperf3 [OK]"
fi

sudo pkill -9 iperf3 2>&1 > /dev/null
rm -fr iperf-*.log
rm -fr port*.log
exit $code
