#!/bin/bash
vagrant destroy -f
vagrant up
sleep 30
vagrant ssh bastion -c 'sudo /vagrant/seagull.sh'
