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
 
    echo -e "\nsctp_test fullnat"
    tail -n 30 sdf.out | grep "Client: Sending packets"
    
    echo -e "\nsctp_test default"
    tail -n 30 sdd.out | grep "Client: Sending packets"

    sfin1=`tail -n 100 sdd.out | grep "Client: Sending packets.(70000/70000)"`
    sfin2=`tail -n 100 sdf.out | grep "Client: Sending packets.(70000/70000)"`
    if [[ ! -z $sfin1 ]]; then
        sdd_res=1
        echo "sctp_test(default) done."
    fi
    if [[ ! -z $sfin2 ]]; then
        sdf_res=1
        echo "sctp_test(fullnat) done."
    fi
    
    if [[ $sdd_res == 1 && $sdf_res == 1 && $iperfd_res == 1 && $iperfd_res == 1 ]]; then
        echo "iperf and sctp_test done."
        break
    fi
    sleep 5
done


pkill iperf
pkill sctp_test
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

if [[ $sdf_res == 1 ]]; then
    echo -e "K8s-calico-ipsec-ha SCTP\t(fullnat)\t[OK]"
else
    echo -e "K8s-calico-ipsec-ha SCTP\t(fullnat)\t[FAILED]"
    code=1
fi

if [[ $sdd_res == 1 ]]; then
    echo -e "K8s-calico-ipsec-ha SCTP\t(default)\t[OK]"
else
    echo -e "K8s-calico-ipsec-ha SCTP\t(default)\t[FAILED]"
    code=1
fi
echo -e "\n\n**********************************************************"
echo $code > /vagrant/status.txt
exit $code
