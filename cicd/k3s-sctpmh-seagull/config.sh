#!/bin/bash
vagrant global-status  | grep -i virtualbox | cut -f 1 -d ' ' | xargs -L 1 vagrant destroy -f
vagrant up
sleep 30
vagrant ssh bastion -c 'sudo /vagrant/seagull.sh'
