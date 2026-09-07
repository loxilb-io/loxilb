#!/bin/bash
set -e
vagrant destroy -f
vagrant up

code=1
for i in {1..60}
do
    if ping 4.0.4.3 -c 1 -W 1 > /dev/null 2>&1; then
        echo -e "Machine rebooted [OK]"
        code=0
        break
    fi
    echo -e "Waiting for machine to be UP"
    sleep 1
done
if [[ $code != 0 ]]; then
    echo "VM not up"
    exit 1
fi
vagrant ssh bastion -c 'sudo /vagrant/setup.sh'
