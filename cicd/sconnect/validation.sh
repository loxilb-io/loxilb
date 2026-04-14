#!/bin/bash
code=0
sizes=( 64 100 500 1000 1500 2000 5000 )

# Prime ARP/ND and bridge FDB for (ns -> target) with retries.
# Retries up to 10 times (~20s) so cold neighbor state recovers from
# INCOMPLETE/FAILED before the measurement ping runs.
warmup_pair() {
    local ns=$1 target=$2 iface=$3
    local i
    for i in 1 2 3 4 5 6 7 8 9 10; do
        if [ -n "$iface" ]; then
            sudo ip netns exec $ns ping $target -c 1 -W 2 -I $iface >/dev/null 2>&1 && return 0
        else
            sudo ip netns exec $ns ping $target -c 1 -W 2 >/dev/null 2>&1 && return 0
        fi
    done
    return 1
}

# Measurement ping: paced (not -f) so fragment reassembly and bridge
# FDB learning are not overwhelmed on slow CI runners. Tolerates up to
# 2 drops out of 10 so transient loss on GitHub runners is not a fail.
measure_ping() {
    local ns=$1 target=$2 iface=$3 size=$4
    local out recv
    if [ -n "$iface" ]; then
        out=$(sudo ip netns exec $ns ping $target -c 10 -i 0.1 -W 2 -s $size -I $iface 2>/dev/null)
    else
        out=$(sudo ip netns exec $ns ping $target -c 10 -i 0.1 -W 2 -s $size 2>/dev/null)
    fi
    recv=$(echo "$out" | awk -F', ' '/packets transmitted/ {print $2}' | awk '{print $1}')
    [ -n "$recv" ] && [ "$recv" -ge 8 ]
}

echo "SCENARIO sconnect"
if [[ $# -gt 0 ]]; then
    nslist=( "$@" )
else
    nslist=( `sudo ip netns | cut -d " " -f 1` )
fi

for ns1 in "${nslist[@]}"; do
    onlyL2=0
    ns1L2=0
	dev1=`sudo ip netns exec $ns1 ip route | grep "default" | cut -d " " -f 5`
    gw1=""
    if [[ -z "$dev1" || "$dev1" == "eth0" ]]
    then
        # No default route present, will try L2 now
        net1=( `sudo ip netns exec $ns1 ip route | grep -v "eth0" | cut -d " " -f 1` )
	    allhosts1=( `sudo ip netns exec $ns1 ip route | grep -v "eth0" cut -d " " -f 9` )
        ns1L2=1
        echo -e "$ns1 can do only L2"
    else
	    hosts1=( `sudo ip netns exec $ns1 ip addr show dev $dev1 | grep -v "eth0" | grep -w inet | cut -d " " -f 6 | cut -d "/" -f 1` )
	    gw1=`sudo ip netns exec $ns1 ip route | grep -v "eth0" | grep "default" | cut -d " " -f 3`
    fi

    if [ ! -z "$gw1" ]
    then
		echo -e "********************************************************************"
		printf "%-16s \t->\t %-16s(Gateway)\n" $ns1 $gw1;
        warmup_pair $ns1 $gw1 ""
        for size in ${sizes[@]}
        do
            measure_ping $ns1 $gw1 "" $size
            if [[ $? -eq 0 ]]
			then
			    #echo -e "Ping [OK]"
				#echo -e "Ping (${hosts1[h1]}) \t->\t ($gw1) \t\t: Packet Size : $size :\t[OK]";
			    printf "Ping %-16s \t->\t %-16s \t\t: Packet Size : %-5s :\t[OK]\n" $ns1 $gw1 $size ;
		    else
			    #echo -e "Ping [NOK]"
				#echo -e "Ping (${hosts1[h1]}) \t->\t ($gw1) \t\t: Packet Size : $size :\t[NOK]";
			    printf "Ping %-16s \t->\t %-16s \t\t: Packet Size : %-5s :\t[NOK]\n" $ns1 $gw1 $size ;
                code=1
	        fi
        done
    fi

	for ns2 in "${nslist[@]}" ; do
		if [ \( "$ns1" = "$ns2" \) -o \( "$ns1" = "loxilb" \) -o \( "$ns2" = "loxilb" \) ]; then 
			continue ;
 		fi;

		echo -e "********************************************************************"
        #Reset onlyL2 flag
        if [ \( $ns1L2 -eq 0 \) -a \( $onlyL2 -eq 1 \) ]
        then
            onlyL2=0
	        hosts1=( `sudo ip netns exec $ns1 ip addr show dev $dev1 | grep -v "eth0" | grep -w inet | cut -d " " -f 6 | cut -d "/" -f 1` )
        fi

		dev2=`sudo ip netns exec $ns2 ip route | grep -v "eth0" | grep "default" | cut -d " " -f 5`
        if [ \( "$dev2" = "" \) -o \( $ns1L2 -eq 1 \) -o \( "$dev2" = "eth0" \) ]
        then
            net1=( `sudo ip netns exec $ns1 ip route | grep -v "eth0" | grep src | cut -d " " -f 1` )
	        allhosts1=( `sudo ip netns exec $ns1 ip route | grep -v "eth0" | grep src | cut -d " " -f 9` )
            onlyL2=1
            echo -e "$ns2 can do only L2 with $ns1"
        fi
		echo -e "$ns1 \t-> $ns2"

        if [ $onlyL2 == 1 ]
        then
            net2=( `sudo ip netns exec $ns2 ip route | grep -v "eth0" | grep src | cut -d " " -f 1` )
	        allhosts2=( `sudo ip netns exec $ns2 ip route | grep -v "eth0" | grep src | cut -d " " -f 9` )
            hosts1=()
            hosts2=()

		    for n1 in "${!net1[@]}"
            do
		        for n2 in "${!net2[@]}"
                do
                    if [ "${net1[n1]}" == "${net2[n2]}" ]
                    then
                        hosts1+=("${allhosts1[$n1]}")
                        hosts2+=("${allhosts2[$n2]}")
                        #echo -e "${net1[n1]}" == "${net2[n2]}"
                        #echo -e "${allhosts1[n1]} and ${allhosts2[n2]}"
                    fi
                done
            done
            for h1 in "${!hosts1[@]}"
            do
                warmup_pair $ns1 ${hosts2[h1]} ""
            done
            for size in ${sizes[@]}
            do
		        for h1 in "${!hosts1[@]}"
		        do
                    measure_ping $ns1 ${hosts2[h1]} "" $size
				    if [[ $? -eq 0 ]]
				    then
				        #echo -e "Ping [OK]"
			            #echo -e "Ping (${hosts1[h1]}) \t->\t (${hosts2[h1]}) \t\t: Packet Size : $size :\t[OK]";
			            printf "Ping %-16s \t->\t %-16s \t\t: Packet Size : %-5s :\t[OK]\n" ${hosts1[h1]} ${hosts2[h1]} $size ;
				    else
				        #echo -e "Ping [NOK]"
			            #echo -e "Ping (${hosts1[h1]}) \t->\t (${hosts2[h1]}) \t\t: Packet Size : $size :\t[NOK]";
			            printf "Ping %-16s \t->\t %-16s \t\t: Packet Size : %-5s :\t[NOK]\n" ${hosts1[h1]} ${hosts2[h1]} $size ;
                        code=1
				    fi
		        done
            done

        else    
		    dev2=`sudo ip netns exec $ns2 ip route | grep -v "eth0" | grep "default" | cut -d " " -f 5`
		    hosts2=( `sudo ip netns exec $ns2 ip addr show dev $dev2 | grep -v "eth0" | grep -w inet | cut -d " " -f 6 | cut -d "/" -f 1` )
            for h1 in "${hosts1[@]}"
            do
                for h2 in "${hosts2[@]}"
                do
                    warmup_pair $ns1 $h2 $h1
                done
            done
            for size in ${sizes[@]}
            do
		        for h1 in "${hosts1[@]}"
		        do
			        for h2 in "${hosts2[@]}"
			        do
                        measure_ping $ns1 $h2 $h1 $size
				        if [[ $? -eq 0 ]]
				        then
					        #echo -e "Ping [OK]"
				            #echo -e "Ping ($h1) \t-> \t($h2) \t\t: Packet Size : $size :\t[OK]";
			                printf "Ping %-16s \t->\t %-16s \t\t: Packet Size : %-5s :\t[OK]\n" $h1 $h2 $size ;
				        else
					        #echo -e "Ping [NOK]"
				            #echo -e "Ping ($h1) \t-> \t($h2) \t\t: Packet Size : $size :\t[NOK]";
			                printf "Ping %-16s \t->\t %-16s \t\t: Packet Size : %-5s :\t[NOK]\n" $h1 $h2 $size ;
                            code=1
				        fi
			        done
		        done
            done
        fi
	done
done
echo -e "********************************************************************"
if [[ $code == 0 ]]
then
    echo SCENARIO-sconnect [OK]
else
    echo SCENARIO-sconnect [FAILED]
fi

exit $code
