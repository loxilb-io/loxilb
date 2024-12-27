#!/bin/bash
VMs=$(vagrant global-status  | grep -i virtualbox)
while IFS= read -a VMs; do
    read -a vm <<< "$VMs"
    cd ${vm[4]} 2>&1>/dev/null
    echo "Destroying ${vm[1]}"
    vagrant destroy -f ${vm[1]}
    cd - 2>&1>/dev/null
done <<< "$VMs"

vagrant up

for((i=1; i<=60; i++))
do
    fin=1
    pods=$(vagrant ssh master -c 'kubectl get pods -A' 2> /dev/null | grep -v "NAMESPACE")

    while IFS= read -a pods; do
        read -a pod <<< "$pods"
        if [[ ${pod[3]} != *"Running"* ]]; then
            echo "${pod[1]} is not UP yet"
            fin=0
        fi
    done <<< "$pods"
    if [ $fin == 1 ];
    then
        break;
    fi
    echo "Will try after 10s"
    sleep 10
done

sudo sysctl net.ipv4.conf.vboxnet1.arp_accept=1

#Create fullnat Service
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/tcp_fullnat.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/udp_fullnat.yml' 2> /dev/null
for((i=1; i<=60; i++))
do
    fin=1
    pods=$(vagrant ssh master -c 'kubectl get pods -A' 2> /dev/null | grep -v "NAMESPACE")

    while IFS= read -a pods; do
        read -a pod <<< "$pods"
        if [[ ${pod[3]} != *"Running"* ]]; then
            echo "${pod[1]} is not UP yet"
            fin=0
        fi
    done <<< "$pods"
    if [ $fin == 1 ];
    then
        echo "Cluster is ready"
        break;
    fi
    echo "Will try after 10s"
    sleep 10
done

if [[ $fin == 0 ]]; then
    echo "Cluster is not ready"
    exit 1
fi
