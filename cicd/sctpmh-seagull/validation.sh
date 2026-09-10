#!/bin/bash
code=0
tc=( "Basic Test - Client & EP Uni-homed and LB is Multi-homed" "Multipath Test, Client and LB Multihomed, EP is uni-homed" "C2LB Multipath Failover Test - Client and LB Multihomed, EP is uni-homed" "E2E Multipath Failover Test - Client, LB and EP all Multihomed" "C2LB HA Failover Test - Client and LB Multihomed, EP is uni-homed" "E2E HA Failover Test. Client, LB and EP all Multihomed" "CT-less Takeover Test - both LBs cold-restarted under a live multipath association" )
padding="............................................................................................................."
border="**************************************************************************************************************************************************"

for((j=0,i=1; i<=7; i++, j++)); do
    echo "SCTP Multihoming - Test case #$i"
    echo -e "\n\n\n$border\n"
    # The VM writes status$i.txt into /vagrant. Start from a clean slate so a
    # case that dies before writing it cannot inherit a stale verdict.
    rm -f status$i.txt
    cmd="sudo /vagrant/validation$i.sh"
    vagrant ssh bastion -c "$cmd"
    echo -e "\n\n"
    file=status$i.txt
    status=$(cat $file 2>/dev/null)
    title=${tc[j]}
    echo -e "\n\n"

    # Anything but an explicit OK is a failure; a missing or empty status file
    # means the case died before reaching a verdict.
    if [[ $status == "OK" ]]; then
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[PASSED]";
    else
        code=1
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[FAILED]";
    fi
    echo -e "\n\n\n$border\n\n"

    sleep 30
done

echo -e "\n\n\n$border\n"
printf "================================================== SCTP MULTIHOMING CONSOLIDATED RESULT ==========================================================\n"
for((j=0,i=1; i<=7; i++, j++)); do
    file=status$i.txt
    status=$(cat $file 2>/dev/null)
    title=${tc[j]}
    echo -e "\n\n"

    # Anything but an explicit OK is a failure; a missing or empty status file
    # means the case died before reaching a verdict.
    if [[ $status == "OK" ]]; then
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[PASSED]";
    else
        code=1
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[FAILED]";
    fi
done

echo -e "\n$border"

echo -e "\n\n\n$border\n"
if [[ $code == 0 ]]; then
    echo -e "SCTP multihoming with seagull CICD [OK]"
else
    echo -e "SCTP Multihoming with seagull CICD [NOK]"
fi
echo -e "\n$border\n"


exit $code
