#!/bin/bash

i="1"     # one second
time=$1
int1=$2   # network interface
int2=$3   # network interface

end=$((SECONDS + $time))

while [ $SECONDS -lt $end ]; do
		txpkts_old1="`cat /sys/class/net/$int1/statistics/tx_packets`" # sent packets
		rxpkts_old1="`cat /sys/class/net/$int1/statistics/rx_packets`" # recv packets
		txpkts_old2="`cat /sys/class/net/$int2/statistics/tx_packets`" # sent packets
		rxpkts_old2="`cat /sys/class/net/$int2/statistics/rx_packets`" # recv packets
        txpkts_old="`expr $txpkts_old1 + $txpkts_old2`"
        rxpkts_old="`expr $rxpkts_old1 + $rxpkts_old2`"
   	    sleep $i
		txpkts_new1="`cat /sys/class/net/$int1/statistics/tx_packets`" # sent packets
        rxpkts_new1="`cat /sys/class/net/$int1/statistics/rx_packets`" # recv packets
		txpkts_new2="`cat /sys/class/net/$int2/statistics/tx_packets`" # sent packets
		rxpkts_new2="`cat /sys/class/net/$int2/statistics/rx_packets`" # recv packets

        txpkts_new="`expr $txpkts_new1 + $txpkts_new2`"
        rxpkts_new="`expr $rxpkts_new1 + $rxpkts_new2`"

		txpkts="`expr $txpkts_new - $txpkts_old`"		     # evaluate expressions for sent packets
		rxpkts="`expr $rxpkts_new - $rxpkts_old`"		     # evaluate expressions for recv packets
		echo "tx $txpkts pkts/s - rx $rxpkts pkts/ on interface $int1 and $int2"
done

