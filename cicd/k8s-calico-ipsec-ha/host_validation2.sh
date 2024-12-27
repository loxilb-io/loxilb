#!/bin/bash
fin=0
for((i=0;i<50;i++))
do
    echo -e "\n --- Host Client status after HA ---\n"
    echo -e "\niperf fullnat"
    tail -n 1 iperff.out

    echo -e "\niperf default"
    tail -n 1 iperfd.out

    ifin1=$(tail -n 5 iperff.out | grep "0.0000-100" | xargs | cut -d ' ' -f 7)
    ifin2=$(tail -n 5 iperff.out | grep "0.0000-100" | xargs | cut -d ' ' -f 7)
    
    if [[ ! -z $ifin1 ]]; then
        iperfd_res=1
        echo "iperfdefault) done."
    fi
    if [[ ! -z $ifin2 ]]; then
        iperff_res=1
        echo "iperf(fullnat) done."
    fi
 
    if [[ $iperfd_res == 1 && $iperfd_res == 1 ]]; then
        echo "iperf done."
        break
    fi
    sleep 5
done


pkill iperf
echo -e "\n\n**********************************************************\n\n"
if [[ $iperff_res == 1 ]]; then
    echo -e "K8s-calico-ipsec-ha TCP\t\t(fullnat)\t[OK]"
else
    echo -e "K8s-calico-ipsec-ha TCP\t\t(fullnat)\t[FAILED]"
    code=1
fi

if [[ $iperfd_res == 1 ]]; then
    echo -e "K8s-calico-ipsec-ha TCP\t\t(default\t[OK]"
else
    echo -e "K8s-calico-ipsec-ha TCP\t\t(default)\t[FAILED]"
    code=1
fi
echo -e "\n\n**********************************************************"
echo $code > /vagrant/status.txt
exit $code
