#!/bin/bash
code=0
sizes=( 64 100 500 1000 1500 2000 5000 )
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
    if [[ -z "$dev1" || "$dev" == "eth0" ]]
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
        for size in ${sizes[@]}
        do

			sudo ip netns exec $ns1 ping $gw1 -f -c 50 -s $size -W 1 2>&1> /dev/null;
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
            for size in ${sizes[@]}
            do
		        for h1 in "${!hosts1[@]}"
		        do
			        #echo -e "(${hosts1[h1]}) \t->\t (${hosts2[h1]}) \t: Packet Size : $size";
                    #echo -e "CMD : sudo ip netns exec $ns1 ping $h2 -f -c 500 -I $h1"
				    sudo ip netns exec $ns1 ping ${hosts2[h1]} -f -c 50 -s $size -W 1 2>&1> /dev/null;
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
            for size in ${sizes[@]}
            do
		        for h1 in "${hosts1[@]}"
		        do
			        for h2 in "${hosts2[@]}"
			        do
				        #echo -e "($h1) -> \t($h2) \t: Packet Size : $size";
                        #echo -e "CMD : sudo ip netns exec $ns1 ping $h2 -f -c 500 -I $h1"
				        sudo ip netns exec $ns1 ping $h2 -f -c 50 -I $h1 -s $size -W 1 2>&1> /dev/null;
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
