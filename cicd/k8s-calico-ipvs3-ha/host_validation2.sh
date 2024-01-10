#!/bin/bash

pkill iperf

iperff_res=$(tail -n 1 iperff.out | xargs | cut -d ' ' -f 7)
iperfd_res=$(tail -n 1 iperff.out | xargs | cut -d ' ' -f 7)

if [[ $iperff_res != 0 ]]; then
    echo -e "K8s-calico-ipvs3-ha TCP\t\t(fullnat)\t[OK]"
else
    echo -e "K8s-calico-ipvs3-ha TCP\t\t(fullnat)\t[FAILED]"
    code=1
fi

if [[ $iperfd_res != 0 ]]; then
    echo -e "K8s-calico-ipvs3-ha TCP\t\t(default\t[OK]"
else
    echo -e "K8s-calico-ipvs3-ha TCP\t\t(default)\t[FAILED]"
    code=1
fi

rm *.out
exit $code
