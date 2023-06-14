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
sudo ip route add 123.123.123.1 via 192.168.90.9

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

#Create default Service
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/tcp.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/udp.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/sctp.yml' 2> /dev/null

#Create onearm Service
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/tcp_onearm.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/udp_onearm.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/sctp_onearm.yml' 2> /dev/null

#Create fullnat Service
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/tcp_fullnat.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/udp_fullnat.yml' 2> /dev/null
vagrant ssh master -c 'kubectl apply -f /vagrant/yaml/sctp_fullnat.yml' 2> /dev/null
