#!/bin/bash
vagrant ssh k3s -c "cd /home/vagrant/loxilb/cicd/k3s-flannel/ && ./validation-k3s.sh"
if [ -f ./error ]; then
  rm -f ./error
  exit 1
fi
exit
