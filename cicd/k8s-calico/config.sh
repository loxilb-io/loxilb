#!/bin/bash
vagrant global-status  | grep -i virtualbox | cut -f 1 -d ' ' | xargs -L 1 vagrant destroy -f
vagrant up
sudo ip route add 123.123.123.1 via 192.168.90.9

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
