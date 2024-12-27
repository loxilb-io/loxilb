#!/bin/bash
for((i=0;i<50;i++))
do
echo "snd=100" 1> sd1.pipe
echo "snd=100" 1> sd2.pipe
sleep 1
done


echo "stats" 1> sd1.pipe
echo "stats" 1> sd2.pipe

echo "shutdown" 1> sd1.pipe
echo "shutdown" 1> sd2.pipe

pkill iperf
pkill sctp_darn

iperff_res=$(tail -n 1 iperff.out | xargs | cut -d ' ' -f 7)
iperfd_res=$(tail -n 1 iperff.out | xargs | cut -d ' ' -f 7)

sdf_res=$(grep -i "Client: Sending packets.(100000/100000)" sdf.out)
sdd_res=$(grep -i "Client: Sending packets.(100000/100000)" sdd.out)

sdf_res1=$(grep -i "packets sent" sdf.out | xargs | cut -d ' ' -f 3)
sdf_res2=$(grep -i "packets rec" sdf.out | xargs | cut -d ' ' -f 3)

sdd_res1=$(grep -i "packets sent" sdd.out | xargs | cut -d ' ' -f 3)
sdd_res2=$(grep -i "packets rec" sdd.out | xargs | cut -d ' ' -f 3)

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

if [[ $sdf_res1 != 0 && $sdf_res2 != 0 && $sdf_res1 == $sdf_res2 ]]; then
    echo -e "K8s-calico-ipvs3-ha SCTP\t(fullnat)\t[OK]"
else
    echo -e "K8s-calico-ipvs3-ha SCTP\t(fullnat)\t[FAILED]"
    code=1
fi

if [[ $sdd_res1 != 0 && $sdd_res2 != 0 && $sdd_res1 == $sdd_res2 ]]; then
    echo -e "K8s-calico-ipvs3-ha SCTP\t(default)\t[OK]"
else
    echo -e "K8s-calico-ipvs3-ha SCTP\t(default)\t[FAILED]"
    code=1
fi
rm *.pipe
rm *.out
exit $code
