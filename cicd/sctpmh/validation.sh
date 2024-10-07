#!/bin/bash
code=0
tc=( "Basic Test - Client & EP Uni-homed and LB is Multi-homed" "Multipath Test, Client and LB Multihomed, EP is uni-homed" "C2LB Multipath Failover Test - Client and LB Multihomed, EP is uni-homed" "E2E Multipath Failover Test - Client, LB and EP all Multihomed" "C2LB HA Failover Test - Client and LB Multihomed, EP is uni-homed" "E2E HA Failover Test. Client, LB and EP all Multihomed" )
padding="............................................................................................................."
border="**************************************************************************************************************************************************"

for((j=0,i=1; i<=6; i++, j++)); do
    echo "SCTP Multihoming - Test case #$i"
    echo -e "\n\n\n$border\n"
    ./validation$i.sh
    echo -e "\n\n"
    file=status$i.txt
    status=`cat $file`
    title=${tc[j]}
    echo -e "\n\n"

    if [[ $status == "NOK" ]]; then
        code=1
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[FAILED]";
    else
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[PASSED]";
    fi
    echo -e "\n\n\n$border\n\n"

    sleep 30
done

echo -e "\n\n\n$border\n"
printf "================================================== SCTP MULTIHOMING CONSOLIDATED RESULT ==========================================================\n"
for((j=0,i=1; i<=6; i++, j++)); do
    file=status$i.txt
    status=`cat $file`
    title=${tc[j]}
    echo -e "\n\n"

    if [[ $status == "NOK" ]]; then
        code=1
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[FAILED]";
    else
        printf "Test case #%2s - %s%s %s\n" "$i" "$title" "${padding:${#title}}" "[PASSED]";
    fi
done

echo -e "\n$border"

echo -e "\n\n\n$border\n"
if [[ $code == 0 ]]; then
    echo -e "SCTP Multihoming with sctp_test CICD [OK]"
else
    echo -e "SCTP Multihoming with sctp_test CICD [NOK]"
fi
echo -e "\n$border\n"

sudo rm -rf statu*.txt
exit $code
