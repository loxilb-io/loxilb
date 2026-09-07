#!/bin/bash
vagrant destroy -f
vagrant up
#vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/tcp-onearm-ds.yml'
#vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/udp-onearm-ds.yml'
#vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/sctp-onearm-ds.yml'
