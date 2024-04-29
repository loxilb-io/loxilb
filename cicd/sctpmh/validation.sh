#!/bin/bash
code=0
echo "SCTP Multihoming - Test case #1"
echo -e "*********************************************************************************"
./validation1.sh
if [[ $? == 1 ]]; then
    code=1
fi
echo -e "\n\n\nSCTP Multihoming - Test case #2"
echo -e "*********************************************************************************"
./validation2.sh
if [[ $? == 1 ]]; then
    code=1
fi
echo -e "\n\n\nSCTP Multihoming - Test case #3"
echo -e "*********************************************************************************"
./validation3.sh
if [[ $? == 1 ]]; then
    code=1
fi
echo -e "\n\n\nSCTP Multihoming - Test case #4"
echo -e "*********************************************************************************"
./validation4.sh
if [[ $? == 1 ]]; then
    code=1
fi
echo -e "\n\n\nSCTP Multihoming - Test case #5"
echo -e "*********************************************************************************"
sleep 60
./validation5.sh
if [[ $? == 1 ]]; then
    code=1
fi
echo -e "\n\n\n*********************************************************************************"
if [[ $code == 0 ]]; then
    echo -e "\n\n SCTP Multihoming CICD [OK]"
else
    echo -e "\n\n SCTP Multihoming CICD [NOK]"
fi
exit $code
