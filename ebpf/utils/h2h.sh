#!/bin/bash
code=0
for ns1 in `sudo ip netns | cut -d " " -f 1`; do
    onlyL2=0
    ns1L2=0
	dev1=`sudo ip netns exec $ns1 ip route | grep "default" | cut -d " " -f 5`
    gw1=""
    if [ -z "$dev1" ]
    then
        # No default route present, will try L2 now
        net1=( `sudo ip netns exec $ns1 ip route | cut -d " " -f 1` )
	    allhosts1=( `sudo ip netns exec $ns1 ip route | cut -d " " -f 9` )
        ns1L2=1
        #echo "$ns1 can do only L2"
    else
	    hosts1=( `sudo ip netns exec $ns1 ip addr show dev $dev1 | grep -w inet | cut -d " " -f 6 | cut -d "/" -f 1` )
	    gw1=`sudo ip netns exec $ns1 ip route | grep "default" | cut -d " " -f 3`
    fi
    
	for ns2 in `sudo ip netns | cut -d " " -f 1`; do
		if [ \( "$ns1" = "$ns2" \) -o \( "$ns1" = "loxilb" \) -o \( "$ns2" = "loxilb" \) ]; then 
			continue ;
 		fi;

		echo "********************************************************************"
        #Reset onlyL2 flag
        if [ \( $ns1L2 -eq 0 \) -a \( $onlyL2 -eq 1 \) ]
        then
            onlyL2=0
	        hosts1=( `sudo ip netns exec $ns1 ip addr show dev $dev1 | grep -w inet | cut -d " " -f 6 | cut -d "/" -f 1` )
        fi

		dev2=`sudo ip netns exec $ns2 ip route | grep "default" | cut -d " " -f 5`
        if [ \( "$dev2" = "" \) -o \( $ns1L2 -eq 1 \) ]
        then
            net1=( `sudo ip netns exec $ns1 ip route | grep src | cut -d " " -f 1` )
	        allhosts1=( `sudo ip netns exec $ns1 ip route | grep src | cut -d " " -f 9` )
            onlyL2=1
            #echo "$ns2 can do only L2 with $ns1"
        fi
		echo "$ns1 -> $ns2"

        if [ $onlyL2 == 1 ]
        then
            net2=( `sudo ip netns exec $ns2 ip route | grep src | cut -d " " -f 1` )
	        allhosts2=( `sudo ip netns exec $ns2 ip route | grep src | cut -d " " -f 9` )
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
                        #echo "${net1[n1]}" == "${net2[n2]}"
                        #echo "${allhosts1[n1]} and ${allhosts2[n2]}"
                    fi
                done
            done
		    for h1 in "${!hosts1[@]}"
		    do
			    echo "(${hosts1[h1]}) -> (${hosts2[h1]})";
                #echo "CMD : sudo ip netns exec $ns1 ping $h2 -f -c 500 -I $h1"
				sudo ip netns exec $ns1 ping ${hosts2[h1]} -f -c 500  2>&1> /dev/null;
				if [[ $? -eq 0 ]]
				then
				    echo "Ping [OK]"
				else
				    echo "Ping [NOK]"
                    code=1
				fi
                if [ ! -z "$gw1" ]
                then
				    echo "(${hosts1[h1]}) -> ($gw1)";
				    sudo ip netns exec $ns1 ping $gw1 -f -c 500 2>&1> /dev/null;
                    if [[ $? -eq 0 ]]
				    then
					    echo "Ping [OK]"
				    else
					    echo "Ping [NOK]"
                        code=1
				    fi

                fi
		    done

        else    
		    dev2=`sudo ip netns exec $ns2 ip route | grep "default" | cut -d " " -f 5`
		    hosts2=( `sudo ip netns exec $ns2 ip addr show dev $dev2 | grep -w inet | cut -d " " -f 6 | cut -d "/" -f 1` )
		    for h1 in "${hosts1[@]}"
		    do
			    for h2 in "${hosts2[@]}"
			    do
				    echo "($h1) -> ($h2)";
                    #echo "CMD : sudo ip netns exec $ns1 ping $h2 -f -c 500 -I $h1"
				    sudo ip netns exec $ns1 ping $h2 -f -c 500 -I $h1 2>&1> /dev/null;
				    if [[ $? -eq 0 ]]
				    then
					    echo "Ping [OK]"
				    else
					    echo "Ping [NOK]"
                        code=1
				    fi
			    done
                if [ ! -z "$gw1" ]
                then
				    echo "($h1) -> ($gw1)";
				    sudo ip netns exec $ns1 ping $gw1 -f -c 500 2>&1> /dev/null;
                    if [[ $? -eq 0 ]]
				    then
					    echo "Ping [OK]"
				    else
					    echo "Ping [NOK]"
                        code=1
				    fi

                fi
		    done
        fi
	done
done
echo "********************************************************************"
exit $code
